// internal/httpserver/router.go
package httpserver

import (
	"embed"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mydelren/context7-proxy/internal/services"
)

type Deps struct {
	MasterKey *services.MasterKeyService
	Keys      *services.KeyService
	Logs      *services.LogService
	Stats     *services.StatsService
	Proxy     *services.ProxyService
	StaticFS  embed.FS
}

func NewRouter(deps Deps) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	mgmt := r.Group("/manage", authMW(deps.MasterKey))
	{
		mgmt.GET("/keys", func(c *gin.Context) {
			keys, err := deps.Keys.List(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, keys)
		})
		mgmt.POST("/keys", func(c *gin.Context) {
			var req struct {
				Key   string `json:"key" binding:"required"`
				Alias string `json:"alias"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "key required"})
				return
			}
			k, err := deps.Keys.Create(c.Request.Context(), req.Key, req.Alias)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, k)
		})
		mgmt.PUT("/keys/:id", func(c *gin.Context) {
			id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
			var req struct {
				Alias    *string `json:"alias"`
				IsActive *bool   `json:"is_active"`
			}
			c.ShouldBindJSON(&req)
			k, err := deps.Keys.Update(c.Request.Context(), uint(id), req.Alias, req.IsActive)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, k)
		})
		mgmt.DELETE("/keys/:id", func(c *gin.Context) {
			id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
			deps.Keys.Delete(c.Request.Context(), uint(id))
			c.JSON(200, gin.H{"ok": true})
		})
		mgmt.GET("/keys/:id/raw", func(c *gin.Context) {
			id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
			key, err := deps.Keys.GetRaw(c.Request.Context(), uint(id))
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"key": key})
		})
		mgmt.DELETE("/keys/invalid", func(c *gin.Context) {
			n, _ := deps.Keys.DeleteInvalid(c.Request.Context())
			c.JSON(200, gin.H{"deleted": n})
		})
		mgmt.GET("/logs", func(c *gin.Context) {
			f := services.LogFilter{Limit: 50}
			if v := c.Query("status_code"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					f.StatusCode = &n
				}
			}
			if v := c.Query("key_id"); v != "" {
				if n, err := strconv.ParseUint(v, 10, 64); err == nil {
					id := uint(n)
					f.KeyID = &id
				}
			}
			if v := c.Query("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					f.Limit = n
				}
			}
			if v := c.Query("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					f.Offset = n
				}
			}
			logs, total, err := deps.Logs.List(c.Request.Context(), f)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"logs": logs, "total": total})
		})
		mgmt.DELETE("/logs", func(c *gin.Context) {
			deps.Logs.Clear(c.Request.Context())
			c.JSON(200, gin.H{"ok": true})
		})
		mgmt.GET("/stats", func(c *gin.Context) {
			stats, err := deps.Stats.Get(c.Request.Context(), deps.Keys.Stats)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, stats)
		})
		mgmt.GET("/stats/timeseries", func(c *gin.Context) {
			hours := 24
			if v := c.Query("hours"); v != "" {
				if n, err := strconv.Atoi(v); err == nil {
					hours = n
				}
			}
			data, err := deps.Stats.TimeSeries(c.Request.Context(), hours)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, data)
		})
		mgmt.GET("/settings/master-key", func(c *gin.Context) {
			c.JSON(200, gin.H{"master_key": deps.MasterKey.Get()})
		})
		mgmt.POST("/settings/master-key/reset", func(c *gin.Context) {
			k, err := deps.MasterKey.Reset(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": "reset_failed"})
				return
			}
			c.JSON(200, gin.H{"master_key": k})
		})
	}

	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/api/") {
			body, _ := c.GetRawData()
			resp, err := deps.Proxy.Do(c.Request.Context(),
				c.Request.Method, path, c.Request.URL.RawQuery,
				c.Request.Header, body, c.ClientIP())
			if err != nil {
				c.JSON(500, gin.H{"error": "proxy_error", "message": err.Error()})
				return
			}
			c.Data(resp.StatusCode, "application/json", resp.Body)
			return
		}
		publicFS, _ := fs.Sub(deps.StaticFS, "static")
		if path == "/" || path == "" {
			c.FileFromFS("/", http.FS(publicFS))
			return
		}
		c.FileFromFS(path, http.FS(publicFS))
	})

	return r
}

func authMW(mk *services.MasterKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if key == "" {
			key = c.Query("api_key")
		}
		if !mk.Validate(key) {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
