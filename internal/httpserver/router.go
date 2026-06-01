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
	Auth     *services.AuthService
	Keys     *services.KeyService
	Logs     *services.LogService
	Stats    *services.StatsService
	Proxy    *services.ProxyService
	StaticFS embed.FS
}

func NewRouter(deps Deps) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	// Public: login
	r.POST("/manage/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "username and password required"})
			return
		}
		token, err := deps.Auth.Login(req.Username, req.Password)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid_credentials"})
			return
		}
		c.JSON(200, gin.H{"token": token, "role": "admin"})
	})

	// Admin-only management routes
	mgmt := r.Group("/manage", authMW(deps.Auth), requireAdmin())
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
			id, ok := parseID(c)
			if !ok {
				return
			}
			var req struct {
				Alias       *string `json:"alias"`
				IsActive    *bool   `json:"is_active"`
				MaxRequests *int64  `json:"max_requests"`
			}
			c.ShouldBindJSON(&req)
			k, err := deps.Keys.Update(c.Request.Context(), id, req.Alias, req.IsActive, req.MaxRequests)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, k)
		})
		mgmt.DELETE("/keys/:id", func(c *gin.Context) {
			id, ok := parseID(c)
			if !ok {
				return
			}
			deps.Keys.Delete(c.Request.Context(), id)
			c.JSON(200, gin.H{"ok": true})
		})
		mgmt.POST("/keys/:id/cooldown", func(c *gin.Context) {
			id, ok := parseID(c)
			if !ok {
				return
			}
			deps.Keys.MarkCooldown(c.Request.Context(), id)
			c.JSON(200, gin.H{"ok": true})
		})
		mgmt.GET("/keys/:id/raw", func(c *gin.Context) {
			id, ok := parseID(c)
			if !ok {
				return
			}
			key, err := deps.Keys.GetRaw(c.Request.Context(), id)
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
			c.JSON(200, gin.H{"master_key": deps.Auth.LegacyKey()})
		})
		mgmt.GET("/settings/strategy", func(c *gin.Context) {
			c.JSON(200, gin.H{"strategy": deps.Keys.GetStrategy(c.Request.Context())})
		})
		mgmt.PUT("/settings/strategy", func(c *gin.Context) {
			var req struct {
				Strategy string `json:"strategy"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "invalid request"})
				return
			}
			if err := deps.Keys.SetStrategy(c.Request.Context(), req.Strategy); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"strategy": req.Strategy})
		})
		mgmt.POST("/settings/change-password", func(c *gin.Context) {
			var req struct {
				OldPassword string `json:"old_password" binding:"required"`
				NewPassword string `json:"new_password" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "old_password and new_password required"})
				return
			}
			// username is always "admin" for now
			if err := deps.Auth.ChangePassword(c.Request.Context(), "admin", req.OldPassword, req.NewPassword); err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"ok": true})
		})
		// Members management
		mgmt.GET("/members", func(c *gin.Context) {
			members, err := deps.Auth.ListMembers(c.Request.Context())
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, members)
		})
		mgmt.POST("/members", func(c *gin.Context) {
			var req struct {
				Name string `json:"name" binding:"required"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "name required"})
				return
			}
			m, err := deps.Auth.CreateMember(c.Request.Context(), req.Name)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"id": m.ID, "name": m.Name, "token": m.Token})
		})
		mgmt.DELETE("/members/:id", func(c *gin.Context) {
			id, ok := parseID(c)
			if !ok {
				return
			}
			if err := deps.Auth.DeleteMember(c.Request.Context(), id); err != nil {
				c.JSON(404, gin.H{"error": "member_not_found"})
				return
			}
			c.JSON(200, gin.H{"ok": true})
		})
		mgmt.PUT("/members/:id", func(c *gin.Context) {
			id, ok := parseID(c)
			if !ok {
				return
			}
			var req struct {
				APIKeyID *uint `json:"api_key_id"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{"error": "invalid request"})
				return
			}
			if err := deps.Auth.UpdateMemberKey(c.Request.Context(), id, req.APIKeyID); err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			c.JSON(200, gin.H{"ok": true})
		})
	}

	// Proxy routes — admin and members can both use
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/api/") {
			// Apply auth only for proxy routes
			token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
			if token == "" {
				token = c.Query("api_key")
			}
			if token == "" {
				c.JSON(401, gin.H{"error": "unauthorized"})
				return
			}
			role, memberID, memberName, apiKeyID := deps.Auth.Validate(token)
			if role == "" {
				c.JSON(401, gin.H{"error": "unauthorized"})
				return
			}
			c.Set("role", role)
			c.Set("member_id", memberID)
			c.Set("member_name", memberName)
			var assignedKeyID uint
			if apiKeyID != nil {
				assignedKeyID = *apiKeyID
			}

			body, _ := c.GetRawData()
			resp, err := deps.Proxy.Do(c.Request.Context(),
				c.Request.Method, path, c.Request.URL.RawQuery,
				c.Request.Header, body, c.ClientIP(),
				memberID, memberName, assignedKeyID)
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

func authMW(a *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			token = c.Query("api_key")
		}
		if token == "" {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		role, memberID, memberName, apiKeyID := a.Validate(token)
		if role == "" {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Set("role", role)
		c.Set("member_id", memberID)
		c.Set("member_name", memberName)
		if apiKeyID != nil {
			c.Set("api_key_id", *apiKeyID)
		}
		c.Next()
	}
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return 0, false
	}
	return uint(id), true
}

func requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("role") != "admin" {
			c.JSON(403, gin.H{"error": "admin_only"})
			c.Abort()
			return
		}
		c.Next()
	}
}
