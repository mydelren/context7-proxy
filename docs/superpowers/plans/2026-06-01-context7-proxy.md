# Context7 API Key Proxy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a lightweight Go proxy that pools multiple Context7 API keys, auto-rotates on 429, and provides a single-file web UI for key management and usage monitoring.

**Architecture:** Single Go binary with embedded web UI. Gin router intercepts Context7 API calls, injects API keys from a SQLite pool, retries on 429 with next available key. Web UI served on the same port.

**Tech Stack:** Go 1.23+ / Gin / GORM / SQLite / embedded single-file HTML+JS (Tailwind CDN)

**Reference:** [TavilyProxyManager](https://github.com/xuncv/TavilyProxyManager)

**Project root:** `/home/mydelren/workspace/oc/context7-proxy/`

---

## File Structure

```
context7-proxy/
├── go.mod
├── go.sum
├── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── db/
│   │   └── db.go
│   ├── models/
│   │   └── models.go
│   ├── services/
│   │   ├── key_service.go
│   │   ├── proxy_service.go
│   │   ├── log_service.go
│   │   ├── stats_service.go
│   │   └── master_key.go
│   └── httpserver/
│       ├── server.go
│       └── router.go
├── static/
│   └── index.html
├── Dockerfile
├── docker-compose.yml
├── .gitignore
└── README.md
```

---

### Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `internal/config/config.go`
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go mod init github.com/mydelren/context7-proxy
```

- [ ] **Step 2: Create config.go**

```go
// internal/config/config.go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr      string
	DatabasePath    string
	Context7BaseURL string
	UpstreamTimeout time.Duration
	CooldownSeconds int
	MasterKey       string
}

func FromEnv() Config {
	return Config{
		ListenAddr:      getEnv("LISTEN_ADDR", ":8070"),
		DatabasePath:    getEnv("DATABASE_PATH", "./data/proxy.db"),
		Context7BaseURL: getEnv("CONTEXT7_BASE_URL", "https://context7.com"),
		UpstreamTimeout: time.Duration(getEnvInt("UPSTREAM_TIMEOUT_SEC", 30)) * time.Second,
		CooldownSeconds: getEnvInt("COOLDOWN_SECONDS", 60),
		MasterKey:       getEnv("MASTER_KEY", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

- [ ] **Step 3: Create main.go skeleton**

```go
// main.go
package main

import (
	"log"
	"github.com/mydelren/context7-proxy/internal/config"
)

func main() {
	cfg := config.FromEnv()
	log.Printf("Context7 Proxy starting on %s", cfg.ListenAddr)
}
```

- [ ] **Step 4: Create .gitignore**

```
/data/
*.db
*.db-journal
context7-proxy
.env
```

- [ ] **Step 5: Verify compilation**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . && rm context7-proxy
```

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: project scaffold with config and main entry point"
```

---

### Task 2: Database Layer

**Files:**
- Create: `internal/models/models.go`
- Create: `internal/db/db.go`

- [ ] **Step 1: Create models.go**

```go
// internal/models/models.go
package models

import "time"

type APIKey struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Key        string     `gorm:"uniqueIndex;not null" json:"-"`
	Alias      string     `gorm:"not null;default:''" json:"alias"`
	IsActive   bool       `gorm:"not null;default:true" json:"is_active"`
	IsInvalid  bool       `gorm:"not null;default:false" json:"is_invalid"`
	CooldownAt *time.Time `json:"cooldown_at"`
	UsedCount  int64      `gorm:"not null;default:0" json:"used_count"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type RequestLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RequestID  string    `gorm:"index;not null" json:"request_id"`
	KeyID      uint      `gorm:"column:key_id;index" json:"key_id"`
	KeyAlias   string    `json:"key_alias"`
	Method     string    `gorm:"not null;default:''" json:"method"`
	Endpoint   string    `gorm:"index;not null" json:"endpoint"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	ClientIP   string    `json:"client_ip"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

type MasterKey struct {
	ID  uint   `gorm:"primaryKey"`
	Key string `gorm:"uniqueIndex;not null"`
}
```

- [ ] **Step 2: Create db.go**

```go
// internal/db/db.go
package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(dbPath string) (*gorm.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	gormDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := gormDB.AutoMigrate(&models.APIKey{}, &models.RequestLog{}, &models.MasterKey{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return gormDB, nil
}
```

- [ ] **Step 3: Update main.go to test DB**

```go
// main.go
package main

import (
	"log"
	"github.com/mydelren/context7-proxy/internal/config"
	"github.com/mydelren/context7-proxy/internal/db"
)

func main() {
	cfg := config.FromEnv()
	log.Printf("Context7 Proxy starting on %s", cfg.ListenAddr)
	_, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	log.Println("Database initialized")
}
```

- [ ] **Step 4: Install deps and verify**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go get gorm.io/gorm gorm.io/driver/sqlite
go mod tidy
go build -o context7-proxy . && ./context7-proxy && rm context7-proxy
```

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add database layer with GORM models and auto-migration"
```

---

### Task 3: Master Key Service

**Files:**
- Create: `internal/services/master_key.go`

- [ ] **Step 1: Create master_key.go**

```go
// internal/services/master_key.go
package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type MasterKeyService struct {
	db        *gorm.DB
	key       string
	customKey string
}

func NewMasterKeyService(db *gorm.DB, customKey string) *MasterKeyService {
	return &MasterKeyService{db: db, customKey: customKey}
}

func (s *MasterKeyService) LoadOrCreate(ctx context.Context) error {
	if s.customKey != "" {
		s.key = s.customKey
		s.db.WithContext(ctx).Where("id = 1").
			Assign(models.MasterKey{Key: s.customKey}).
			FirstOrCreate(&models.MasterKey{ID: 1, Key: s.customKey})
		return nil
	}
	var mk models.MasterKey
	if err := s.db.WithContext(ctx).First(&mk).Error; err == nil {
		s.key = mk.Key
		return nil
	}
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate master key: %w", err)
	}
	s.key = hex.EncodeToString(b)
	return s.db.WithContext(ctx).Create(&models.MasterKey{ID: 1, Key: s.key}).Error
}

func (s *MasterKeyService) Get() string    { return s.key }
func (s *MasterKeyService) Validate(k string) bool { return k == s.key }

func (s *MasterKeyService) Reset(ctx context.Context) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	s.key = hex.EncodeToString(b)
	return s.key, s.db.WithContext(ctx).Model(&models.MasterKey{}).
		Where("id = 1").Update("key", s.key).Error
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . 2>&1 && rm context7-proxy
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add master key service with auto-generation"
```

---

### Task 4: Key Service

**Files:**
- Create: `internal/services/key_service.go`

- [ ] **Step 1: Create key_service.go**

```go
// internal/services/key_service.go
package services

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type KeyService struct {
	db          *gorm.DB
	cooldownSec int
}

func NewKeyService(db *gorm.DB, cooldownSec int) *KeyService {
	return &KeyService{db: db, cooldownSec: cooldownSec}
}

func (s *KeyService) List(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	return keys, s.db.WithContext(ctx).Order("id desc").Find(&keys).Error
}

func (s *KeyService) Create(ctx context.Context, key, alias string) (*models.APIKey, error) {
	k := models.APIKey{Key: key, Alias: alias, IsActive: true}
	return &k, s.db.WithContext(ctx).Create(&k).Error
}

func (s *KeyService) Update(ctx context.Context, id uint, alias *string, isActive *bool) (*models.APIKey, error) {
	var k models.APIKey
	if err := s.db.WithContext(ctx).First(&k, id).Error; err != nil {
		return nil, err
	}
	if alias != nil {
		k.Alias = *alias
	}
	if isActive != nil {
		if k.IsInvalid && *isActive {
			k.IsActive = false
		} else {
			k.IsActive = *isActive
		}
	}
	return &k, s.db.WithContext(ctx).Save(&k).Error
}

func (s *KeyService) Delete(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.APIKey{}, id).Error
}

func (s *KeyService) GetRaw(ctx context.Context, id uint) (string, error) {
	var k models.APIKey
	err := s.db.WithContext(ctx).Select("key").First(&k, id).Error
	return k.Key, err
}

type KeyCandidate struct {
	ID        uint
	Key       string
	Alias     string
	UsedCount int64
}

func (s *KeyService) Candidates(ctx context.Context) ([]KeyCandidate, error) {
	var keys []models.APIKey
	now := time.Now()
	if err := s.db.WithContext(ctx).
		Where("is_invalid = ? AND is_active = ?", false, true).
		Find(&keys).Error; err != nil {
		return nil, err
	}
	var out []KeyCandidate
	for _, k := range keys {
		if k.CooldownAt != nil && k.CooldownAt.After(now) {
			continue
		}
		out = append(out, KeyCandidate{ID: k.ID, Key: k.Key, Alias: k.Alias, UsedCount: k.UsedCount})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UsedCount != out[j].UsedCount {
			return out[i].UsedCount < out[j].UsedCount
		}
		return rand.Intn(2) == 0
	})
	return out, nil
}

func (s *KeyService) MarkUsed(ctx context.Context, id uint) {
	now := time.Now()
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).
		Updates(map[string]interface{}{"used_count": gorm.Expr("used_count + 1"), "last_used_at": now})
}

func (s *KeyService) MarkCooldown(ctx context.Context, id uint) {
	cd := time.Now().Add(time.Duration(s.cooldownSec) * time.Second)
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).Update("cooldown_at", cd)
}

func (s *KeyService) MarkInvalid(ctx context.Context, id uint) {
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("id = ?", id).
		Updates(map[string]interface{}{"is_invalid": true, "is_active": false})
}

func (s *KeyService) DeleteInvalid(ctx context.Context) (int64, error) {
	r := s.db.WithContext(ctx).Where("is_invalid = ?", true).Delete(&models.APIKey{})
	return r.RowsAffected, r.Error
}

func (s *KeyService) Stats(ctx context.Context) (total, active, cooling, invalid int, err error) {
	var keys []models.APIKey
	if err = s.db.WithContext(ctx).Find(&keys).Error; err != nil {
		return
	}
	now := time.Now()
	for _, k := range keys {
		total++
		if k.IsInvalid {
			invalid++
		} else if k.IsActive {
			if k.CooldownAt != nil && k.CooldownAt.After(now) {
				cooling++
			}
			active++
		}
	}
	return
}
```

- [ ] **Step 2: Verify**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . 2>&1 && rm context7-proxy
```

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add key service with CRUD and 429-aware selection"
```

---

### Task 5: Log + Stats Services

**Files:**
- Create: `internal/services/log_service.go`
- Create: `internal/services/stats_service.go`

- [ ] **Step 1: Create log_service.go**

```go
// internal/services/log_service.go
package services

import (
	"context"
	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type LogService struct{ db *gorm.DB }

func NewLogService(db *gorm.DB) *LogService { return &LogService{db: db} }

func (s *LogService) Create(ctx context.Context, l *models.RequestLog) error {
	return s.db.WithContext(ctx).Create(l).Error
}

type LogFilter struct {
	StatusCode *int
	KeyID      *uint
	Limit      int
	Offset     int
}

func (s *LogService) List(ctx context.Context, f LogFilter) ([]models.RequestLog, int64, error) {
	q := s.db.WithContext(ctx).Model(&models.RequestLog{})
	if f.StatusCode != nil {
		q = q.Where("status_code = ?", *f.StatusCode)
	}
	if f.KeyID != nil {
		q = q.Where("key_id = ?", *f.KeyID)
	}
	var total int64
	q.Count(&total)
	if f.Limit <= 0 {
		f.Limit = 50
	}
	var logs []models.RequestLog
	err := q.Order("id desc").Offset(f.Offset).Limit(f.Limit).Find(&logs).Error
	return logs, total, err
}

func (s *LogService) Clear(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&models.RequestLog{}).Error
}
```

- [ ] **Step 2: Create stats_service.go**

```go
// internal/services/stats_service.go
package services

import (
	"context"
	"time"

	"github.com/mydelren/context7-proxy/internal/models"
	"gorm.io/gorm"
)

type StatsService struct{ db *gorm.DB }

func NewStatsService(db *gorm.DB) *StatsService { return &StatsService{db: db} }

type Stats struct {
	TotalRequests int64 `json:"total_requests"`
	TodayRequests int64 `json:"today_requests"`
	RateLimited   int64 `json:"rate_limited"`
	TotalKeys     int   `json:"total_keys"`
	ActiveKeys    int   `json:"active_keys"`
	CoolingKeys   int   `json:"cooling_keys"`
	InvalidKeys   int   `json:"invalid_keys"`
}

func (s *StatsService) Get(ctx context.Context, keyStats func(context.Context) (int, int, int, int, error)) (Stats, error) {
	var st Stats
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Count(&st.TotalRequests)
	today := time.Now().Truncate(24 * time.Hour)
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Where("created_at >= ?", today).Count(&st.TodayRequests)
	s.db.WithContext(ctx).Model(&models.RequestLog{}).Where("status_code = ?", 429).Count(&st.RateLimited)
	t, a, c, i, err := keyStats(ctx)
	st.TotalKeys, st.ActiveKeys, st.CoolingKeys, st.InvalidKeys = t, a, c, i
	return st, err
}

type TimeSeriesPoint struct {
	Hour  string `json:"hour"`
	Count int64  `json:"count"`
}

func (s *StatsService) TimeSeries(ctx context.Context, hours int) ([]TimeSeriesPoint, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	rows, err := s.db.WithContext(ctx).Raw(`
		SELECT strftime('%Y-%m-%d %H:00', created_at, 'localtime') as hour, count(*) as count
		FROM request_logs WHERE created_at >= ? GROUP BY hour ORDER BY hour
	`, since).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pts []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		rows.Scan(&p.Hour, &p.Count)
		pts = append(pts, p)
	}
	return pts, nil
}
```

- [ ] **Step 3: Verify and commit**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . 2>&1 && rm context7-proxy
git add -A && git commit -m "feat: add log and stats services"
```

---

### Task 6: Core Proxy Service

**Files:**
- Create: `internal/services/proxy_service.go`

- [ ] **Step 1: Create proxy_service.go**

```go
// internal/services/proxy_service.go
package services

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mydelren/context7-proxy/internal/models"
)

type ProxyService struct {
	baseURL string
	client  *http.Client
	keys    *KeyService
	logs    *LogService
}

func NewProxyService(baseURL string, timeout time.Duration, keys *KeyService, logs *LogService) *ProxyService {
	return &ProxyService{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
		keys:    keys,
		logs:    logs,
	}
}

type ProxyResponse struct {
	StatusCode int
	Body       []byte
}

func (p *ProxyService) Do(ctx context.Context, method, path, rawQuery string, headers http.Header, body []byte, clientIP string) (ProxyResponse, error) {
	reqID := uuid.NewString()[:8]
	candidates, err := p.keys.Candidates(ctx)
	if err != nil {
		return ProxyResponse{}, err
	}
	if len(candidates) == 0 {
		return ProxyResponse{StatusCode: 503, Body: []byte(`{"error":"no_available_keys"}`)}, nil
	}

	var lastResp ProxyResponse
	for _, c := range candidates {
		respBody, status, latency, err := p.tryKey(ctx, c.Key, method, path, rawQuery, headers, body)
		if err != nil {
			log.Printf("[%s] key %d (%s) error: %v", reqID, c.ID, c.Alias, err)
			continue
		}
		switch status {
		case 401:
			log.Printf("[%s] key %d (%s) invalid (401)", reqID, c.ID, c.Alias)
			p.keys.MarkInvalid(ctx, c.ID)
			continue
		case 429:
			log.Printf("[%s] key %d (%s) rate limited (429)", reqID, c.ID, c.Alias)
			p.keys.MarkCooldown(ctx, c.ID)
			lastResp = ProxyResponse{StatusCode: 429, Body: respBody}
			continue
		}
		p.keys.MarkUsed(ctx, c.ID)
		p.logReq(ctx, reqID, c.ID, c.Alias, method, path, status, latency, clientIP)
		return ProxyResponse{StatusCode: status, Body: respBody}, nil
	}

	if lastResp.StatusCode == 429 {
		p.logReq(ctx, reqID, 0, "", method, path, 429, 0, clientIP)
		return lastResp, nil
	}
	return ProxyResponse{StatusCode: 503, Body: []byte(`{"error":"all_keys_failed"}`)}, nil
}

func (p *ProxyService) tryKey(ctx context.Context, apiKey, method, path, rawQuery string, headers http.Header, body []byte) ([]byte, int, int64, error) {
	u := p.baseURL + path
	if rawQuery != "" {
		u += "?" + rawQuery
	}
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, r)
	if err != nil {
		return nil, 0, 0, err
	}
	for k, vs := range headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	start := time.Now()
	resp, err := p.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, 0, latency, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, latency, err
}

func (p *ProxyService) logReq(ctx context.Context, reqID string, keyID uint, alias, method, endpoint string, status int, latency int64, clientIP string) {
	if p.logs == nil {
		return
	}
	p.logs.Create(ctx, &models.RequestLog{
		RequestID: reqID, KeyID: keyID, KeyAlias: alias,
		Method: method, Endpoint: endpoint, StatusCode: status, LatencyMs: latency,
		ClientIP: clientIP, CreatedAt: time.Now(),
	})
}
```

- [ ] **Step 2: Install uuid and verify**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go get github.com/google/uuid
go mod tidy
go build -o context7-proxy . 2>&1 && rm context7-proxy
```

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: add core proxy service with key rotation and 429 retry"
```

---

### Task 7: HTTP Router + Server

**Files:**
- Create: `internal/httpserver/router.go`
- Create: `internal/httpserver/server.go`

- [ ] **Step 1: Create router.go**

```go
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
```

- [ ] **Step 2: Create server.go**

```go
// internal/httpserver/server.go
package httpserver

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func Run(addr string, handler http.Handler) {
	srv := &http.Server{Addr: addr, Handler: handler, ReadTimeout: 60 * time.Second, WriteTimeout: 60 * time.Second}
	go func() {
		log.Printf("Listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
```

- [ ] **Step 3: Install gin and verify**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go get github.com/gin-gonic/gin
go mod tidy
go build -o context7-proxy . 2>&1 && rm context7-proxy
```

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add HTTP router with proxy, management API, and static serving"
```

---

### Task 8: Wire main.go

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Rewrite main.go**

```go
// main.go
package main

import (
	"embed"
	"log"

	"github.com/mydelren/context7-proxy/internal/config"
	"github.com/mydelren/context7-proxy/internal/db"
	"github.com/mydelren/context7-proxy/internal/httpserver"
	"github.com/mydelren/context7-proxy/internal/services"
)

//go:embed static/*
var staticFS embed.FS

func main() {
	cfg := config.FromEnv()

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	mkSvc := services.NewMasterKeyService(database, cfg.MasterKey)
	if err := mkSvc.LoadOrCreate(nil); err != nil {
		log.Fatalf("Master key init failed: %v", err)
	}
	if cfg.MasterKey == "" {
		log.Printf("Generated master key: %s", mkSvc.Get())
		log.Println("Save this key! You need it for the management UI.")
	}

	keySvc := services.NewKeyService(database, cfg.CooldownSeconds)
	logSvc := services.NewLogService(database)
	statsSvc := services.NewStatsService(database)
	proxySvc := services.NewProxyService(cfg.Context7BaseURL, cfg.UpstreamTimeout, keySvc, logSvc)

	httpserver.Run(cfg.ListenAddr, httpserver.NewRouter(httpserver.Deps{
		MasterKey: mkSvc, Keys: keySvc, Logs: logSvc,
		Stats: statsSvc, Proxy: proxySvc, StaticFS: staticFS,
	}))
}
```

- [ ] **Step 2: Create placeholder for embed**

```bash
mkdir -p /home/mydelren/workspace/oc/context7-proxy/static
echo '<h1>Context7 Proxy</h1>' > /home/mydelren/workspace/oc/context7-proxy/static/index.html
```

- [ ] **Step 3: Verify full build**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . 2>&1 && rm context7-proxy
```

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: wire all services in main.go with embed.FS"
```

---

### Task 9: Web UI

**Files:**
- Modify: `static/index.html`

- [ ] **Step 1: Write the full index.html**

Write the complete single-file web UI to `static/index.html`:

```html
<!DOCTYPE html>
<html lang="en" class="dark">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Context7 Proxy</title>
<script src="https://cdn.tailwindcss.com"></script>
<script src="https://cdn.jsdelivr.net/npm/chart.js@4"></script>
<script>tailwindcss.config={darkMode:'class'}</script>
<style>body{font-family:system-ui,-apple-system,sans-serif}</style>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen">

<!-- Auth Screen -->
<div id="authScreen" class="flex items-center justify-center min-h-screen">
  <div class="bg-gray-900 p-8 rounded-xl shadow-lg w-full max-w-sm">
    <h1 class="text-xl font-bold mb-4 text-center">Context7 Proxy</h1>
    <input id="masterKeyInput" type="password" placeholder="Master Key"
      class="w-full px-3 py-2 bg-gray-800 rounded border border-gray-700 focus:outline-none focus:border-blue-500 mb-3">
    <button onclick="login()" class="w-full py-2 bg-blue-600 hover:bg-blue-700 rounded font-medium">Login</button>
    <p id="authError" class="text-red-400 text-sm mt-2 hidden"></p>
  </div>
</div>

<!-- Main App -->
<div id="app" class="hidden">
  <!-- Nav -->
  <nav class="bg-gray-900 border-b border-gray-800 px-4 py-3 flex items-center justify-between">
    <div class="flex gap-4">
      <button onclick="switchTab('dashboard')" class="nav-btn font-medium" data-tab="dashboard">Dashboard</button>
      <button onclick="switchTab('keys')" class="nav-btn font-medium" data-tab="keys">Keys</button>
      <button onclick="switchTab('logs')" class="nav-btn font-medium" data-tab="logs">Logs</button>
      <button onclick="switchTab('settings')" class="nav-btn font-medium" data-tab="settings">Settings</button>
    </div>
    <button onclick="logout()" class="text-sm text-gray-400 hover:text-gray-200">Logout</button>
  </nav>

  <main class="max-w-6xl mx-auto p-4">

    <!-- Dashboard -->
    <section id="tab-dashboard">
      <div class="grid grid-cols-2 md:grid-cols-5 gap-3 mb-6" id="statCards"></div>
      <div class="bg-gray-900 rounded-xl p-4">
        <h3 class="text-sm font-medium text-gray-400 mb-3">Requests (last 24h)</h3>
        <canvas id="chart" height="80"></canvas>
      </div>
    </section>

    <!-- Keys -->
    <section id="tab-keys" class="hidden">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold">API Keys</h2>
        <button onclick="showKeyModal()" class="px-3 py-1.5 bg-blue-600 hover:bg-blue-700 rounded text-sm">Add Key</button>
      </div>
      <div class="overflow-x-auto"><table class="w-full text-sm" id="keysTable">
        <thead><tr class="text-gray-400 border-b border-gray-800">
          <th class="text-left p-2">Alias</th><th class="text-left p-2">Status</th>
          <th class="text-right p-2">Used</th><th class="text-left p-2">Last Used</th>
          <th class="text-right p-2">Actions</th>
        </tr></thead><tbody></tbody>
      </table></div>
    </section>

    <!-- Logs -->
    <section id="tab-logs" class="hidden">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-semibold">Request Logs</h2>
        <div class="flex gap-2">
          <select id="logFilter" onchange="loadLogs(1)" class="bg-gray-800 border border-gray-700 rounded px-2 py-1 text-sm">
            <option value="">All</option><option value="200">200</option><option value="429">429</option>
            <option value="401">401</option><option value="500">500</option>
          </select>
          <button onclick="clearLogs()" class="px-3 py-1.5 bg-red-600/80 hover:bg-red-700 rounded text-sm">Clear</button>
        </div>
      </div>
      <div class="overflow-x-auto"><table class="w-full text-sm" id="logsTable">
        <thead><tr class="text-gray-400 border-b border-gray-800">
          <th class="text-left p-2">Time</th><th class="text-left p-2">Method</th>
          <th class="text-left p-2">Path</th><th class="text-right p-2">Status</th>
          <th class="text-right p-2">Latency</th>
        </tr></thead><tbody></tbody>
      </table></div>
      <div class="flex justify-center gap-2 mt-4" id="logPagination"></div>
    </section>

    <!-- Settings -->
    <section id="tab-settings" class="hidden">
      <div class="bg-gray-900 rounded-xl p-6 max-w-xl">
        <h2 class="text-lg font-semibold mb-4">Settings</h2>
        <div class="mb-6">
          <label class="text-sm text-gray-400 block mb-1">Master Key</label>
          <div class="flex gap-2">
            <input id="masterKeyDisplay" readonly class="flex-1 px-3 py-2 bg-gray-800 rounded border border-gray-700 font-mono text-sm">
            <button onclick="copyMasterKey()" class="px-3 py-2 bg-gray-700 hover:bg-gray-600 rounded text-sm">Copy</button>
            <button onclick="resetMasterKey()" class="px-3 py-2 bg-red-600/80 hover:bg-red-700 rounded text-sm">Reset</button>
          </div>
        </div>
        <div>
          <label class="text-sm text-gray-400 block mb-1">MCP Client Config</label>
          <pre class="bg-gray-800 rounded p-3 text-xs overflow-x-auto" id="mcpConfig"></pre>
        </div>
      </div>
    </section>
  </main>
</div>

<!-- Add Key Modal -->
<div id="keyModal" class="fixed inset-0 bg-black/60 flex items-center justify-center hidden z-50">
  <div class="bg-gray-900 p-6 rounded-xl shadow-lg w-full max-w-md">
    <h3 class="font-semibold mb-3">Add API Key</h3>
    <input id="newKeyAlias" placeholder="Alias (optional)" class="w-full px-3 py-2 bg-gray-800 rounded border border-gray-700 mb-2">
    <input id="newKeyValue" placeholder="ctx7sk_..." class="w-full px-3 py-2 bg-gray-800 rounded border border-gray-700 mb-3">
    <div class="flex justify-end gap-2">
      <button onclick="hideKeyModal()" class="px-4 py-2 bg-gray-700 hover:bg-gray-600 rounded text-sm">Cancel</button>
      <button onclick="addKey()" class="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm">Add</button>
    </div>
  </div>
</div>

<script>
let TOKEN = localStorage.getItem('c7p_token') || '';
let chart = null;

// API helpers
const api = {
  stats: () => fetch('/manage/stats', { headers: auth() }).then(r => r.json()),
  timeseries: () => fetch('/manage/stats/timeseries', { headers: auth() }).then(r => r.json()),
  keys: () => fetch('/manage/keys', { headers: auth() }).then(r => r.json()),
  addKey: (alias, key) => fetch('/manage/keys', { method: 'POST', headers: { ...auth(), 'Content-Type': 'application/json' }, body: JSON.stringify({ alias, key }) }).then(r => r.json()),
  delKey: id => fetch('/manage/keys/' + id, { method: 'DELETE', headers: auth() }).then(r => r.json()),
  toggleKey: (id, active) => fetch('/manage/keys/' + id, { method: 'PUT', headers: { ...auth(), 'Content-Type': 'application/json' }, body: JSON.stringify({ is_active: active }) }).then(r => r.json()),
  logs: (p, f) => { const off = (p - 1) * 50; return fetch('/manage/logs?limit=50&offset=' + off + (f ? '&status_code=' + f : ''), { headers: auth() }).then(r => r.json()); },
  clearLogs: () => fetch('/manage/logs', { method: 'DELETE', headers: auth() }).then(r => r.json()),
  resetKey: () => fetch('/manage/settings/master-key/reset', { method: 'POST', headers: auth() }).then(r => r.json()),
};

function auth() { return { 'Authorization': 'Bearer ' + TOKEN }; }

function fmtTime(ts) { return ts ? new Date(ts).toLocaleString() : '-'; }
function fmtMs(ms) { return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms'; }

// Auth
async function login() {
  const key = document.getElementById('masterKeyInput').value.trim();
  if (!key) return;
  TOKEN = key;
  try {
    await api.stats();
    localStorage.setItem('c7p_token', key);
    showApp();
  } catch {
    document.getElementById('authError').textContent = 'Invalid master key';
    document.getElementById('authError').classList.remove('hidden');
    TOKEN = '';
  }
}

function logout() {
  TOKEN = '';
  localStorage.removeItem('c7p_token');
  document.getElementById('app').classList.add('hidden');
  document.getElementById('authScreen').classList.remove('hidden');
}

async function showApp() {
  document.getElementById('authScreen').classList.add('hidden');
  document.getElementById('app').classList.remove('hidden');
  document.getElementById('masterKeyDisplay').value = TOKEN;
  const host = location.origin;
  document.getElementById('mcpConfig').textContent = JSON.stringify({
    mcpServers: { context7: { command: 'npx', args: ['-y', '@upstash/context7-mcp@latest'],
      env: { CONTEXT7_API_URL: host } } }
  }, null, 2);
  switchTab('dashboard');
}

// Tabs
function switchTab(name) {
  ['dashboard', 'keys', 'logs', 'settings'].forEach(t => {
    document.getElementById('tab-' + t).classList.toggle('hidden', t !== name);
    document.querySelectorAll('[data-tab="' + t + '"]').forEach(b =>
      b.classList.toggle('text-blue-400', t === name));
  });
  if (name === 'dashboard') loadDashboard();
  if (name === 'keys') loadKeys();
  if (name === 'logs') loadLogs(1);
}

// Dashboard
async function loadDashboard() {
  const [s, ts] = await Promise.all([api.stats(), api.timeseries()]);
  const cards = [
    { label: 'Total Keys', value: s.total_keys, color: 'blue' },
    { label: 'Active Keys', value: s.active_keys, color: 'green' },
    { label: 'Cooling Down', value: s.cooling_keys, color: 'yellow' },
    { label: 'Invalid Keys', value: s.invalid_keys, color: 'red' },
    { label: 'Requests (24h)', value: s.today_requests, color: 'purple' },
  ];
  document.getElementById('statCards').innerHTML = cards.map(c =>
    `<div class="bg-gray-900 rounded-xl p-4"><div class="text-sm text-gray-400">${c.label}</div><div class="text-2xl font-bold text-${c.color}-400 mt-1">${c.value}</div></div>`
  ).join('');
  const labels = ts.map(t => t.hour);
  const data = ts.map(t => t.count);
  if (chart) chart.destroy();
  chart = new Chart(document.getElementById('chart'), {
    type: 'bar', data: { labels, datasets: [{ label: 'Requests', data, backgroundColor: '#3b82f6', borderRadius: 4 }] },
    options: { responsive: true, plugins: { legend: { display: false } }, scales: { x: { ticks: { color: '#9ca3af' } }, y: { ticks: { color: '#9ca3af' }, beginAtZero: true } } }
  });
}

// Keys
async function loadKeys() {
  const keys = await api.keys();
  const tbody = document.querySelector('#keysTable tbody');
  tbody.innerHTML = keys.map(k => `<tr class="border-b border-gray-800 hover:bg-gray-900/50">
    <td class="p-2">${k.alias || '-'}</td>
    <td class="p-2"><span class="px-2 py-0.5 rounded text-xs ${k.is_invalid ? 'bg-red-900 text-red-300' : k.is_active ? (k.cooldown_at ? 'bg-yellow-900 text-yellow-300' : 'bg-green-900 text-green-300') : 'bg-gray-700 text-gray-400'}">${k.is_invalid ? 'Invalid' : k.is_active ? (k.cooldown_at ? 'Cooling' : 'Active') : 'Disabled'}</span></td>
    <td class="p-2 text-right">${k.used_count}</td><td class="p-2">${fmtTime(k.last_used_at)}</td>
    <td class="p-2 text-right"><button onclick="toggleKey(${k.id},${!k.is_active})" class="text-xs px-2 py-1 rounded ${k.is_active ? 'bg-gray-700 hover:bg-gray-600' : 'bg-green-800 hover:bg-green-700'}">${k.is_active ? 'Disable' : 'Enable'}</button>
    <button onclick="delKey(${k.id})" class="text-xs px-2 py-1 rounded bg-red-800 hover:bg-red-700 ml-1">Delete</button></td>
  </tr>`).join('');
}

function showKeyModal() { document.getElementById('keyModal').classList.remove('hidden'); }
function hideKeyModal() { document.getElementById('keyModal').classList.add('hidden'); document.getElementById('newKeyAlias').value = ''; document.getElementById('newKeyValue').value = ''; }

async function addKey() {
  const alias = document.getElementById('newKeyAlias').value.trim();
  const key = document.getElementById('newKeyValue').value.trim();
  if (!key) return;
  await api.addKey(alias, key);
  hideKeyModal();
  loadKeys();
}

async function toggleKey(id, active) { await api.toggleKey(id, active); loadKeys(); }
async function delKey(id) { if (confirm('Delete this key?')) { await api.delKey(id); loadKeys(); } }

// Logs
async function loadLogs(page) {
  const filter = document.getElementById('logFilter').value;
  const res = await api.logs(page, filter);
  const tbody = document.querySelector('#logsTable tbody');
  tbody.innerHTML = res.logs.map(l => `<tr class="border-b border-gray-800 hover:bg-gray-900/50">
    <td class="p-2">${fmtTime(l.created_at)}</td><td class="p-2">${l.method}</td>
    <td class="p-2 font-mono text-xs">${l.endpoint}</td>
    <td class="p-2 text-right"><span class="${l.status_code >= 400 ? 'text-red-400' : 'text-green-400'}">${l.status_code}</span></td>
    <td class="p-2 text-right">${fmtMs(l.latency_ms)}</td>
  </tr>`).join('');
  const totalPages = Math.ceil(res.total / 50);
  let pg = '';
  for (let i = 1; i <= totalPages; i++) {
    pg += `<button onclick="loadLogs(${i})" class="px-3 py-1 rounded text-sm ${i === page ? 'bg-blue-600' : 'bg-gray-800 hover:bg-gray-700'}">${i}</button>`;
  }
  document.getElementById('logPagination').innerHTML = pg;
}

async function clearLogs() { if (confirm('Clear all logs?')) { await api.clearLogs(); loadLogs(1); } }

// Settings
async function resetMasterKey() {
  if (!confirm('Reset master key? You will need the new key to log in.')) return;
  const res = await api.resetKey();
  TOKEN = res.master_key;
  localStorage.setItem('c7p_token', TOKEN);
  document.getElementById('masterKeyDisplay').value = TOKEN;
  alert('New master key: ' + TOKEN + '\nSave it!');
}

function copyMasterKey() { navigator.clipboard.writeText(document.getElementById('masterKeyDisplay').value); }

// Auto-login on load
if (TOKEN) { api.stats().then(() => showApp()).catch(() => { TOKEN = ''; localStorage.removeItem('c7p_token'); }); }
</script>
</body>
</html>
```

- [ ] **Step 2: Verify build embeds the file**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
go build -o context7-proxy . 2>&1 && ls -la context7-proxy && rm context7-proxy
```

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "feat: add single-file web UI with dashboard, keys, logs, settings"
```

---

### Task 10: Docker + README

**Files:**
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Create: `README.md`

- [ ] **Step 1: Create Dockerfile**

```dockerfile
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o context7-proxy .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/context7-proxy .
VOLUME /app/data
ENV DATABASE_PATH=/app/data/proxy.db
EXPOSE 8070
CMD ["./context7-proxy"]
```

- [ ] **Step 2: Create docker-compose.yml**

```yaml
services:
  context7-proxy:
    build: .
    ports:
      - "8070:8070"
    environment:
      - DATABASE_PATH=/app/data/proxy.db
      - CONTEXT7_BASE_URL=https://context7.com
      - UPSTREAM_TIMEOUT_SEC=30
      - COOLDOWN_SECONDS=60
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

- [ ] **Step 3: Create README.md**

```markdown
# Context7 API Key Proxy

Pool multiple Context7 API keys behind a single endpoint. Auto-rotates on 429 rate limits. Web management UI included.

## Quick Start

```bash
docker compose up -d
docker compose logs | grep "master key"
```

Open `http://localhost:8070`, enter the master key, add your `ctx7sk_...` keys.

## MCP Client Config

```json
{
  "mcpServers": {
    "context7": {
      "command": "npx",
      "args": ["-y", "@upstash/context7-mcp@latest"],
      "env": {
        "CONTEXT7_API_URL": "http://127.0.0.1:8070"
      }
    }
  }
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8070` | Listen address |
| `DATABASE_PATH` | `./data/proxy.db` | SQLite path |
| `CONTEXT7_BASE_URL` | `https://context7.com` | Upstream URL |
| `UPSTREAM_TIMEOUT_SEC` | `30` | Timeout (sec) |
| `COOLDOWN_SECONDS` | `60` | 429 cooldown |
| `MASTER_KEY` | auto | Custom master key |

## License

MIT
```

- [ ] **Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Dockerfile, docker-compose, and README"
```

---

### Task 11: Build and Verify

- [ ] **Step 1: Docker build**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
docker compose build 2>&1
```

- [ ] **Step 2: Start and get master key**

```bash
docker compose up -d
docker compose logs | grep "master key"
```

- [ ] **Step 3: Test health**

```bash
curl -s http://localhost:8070/healthz
# Expected: {"ok":true}
```

- [ ] **Step 4: Test management API**

```bash
MASTER_KEY="<from step 2>"
curl -s -H "Authorization: Bearer $MASTER_KEY" http://localhost:8070/manage/keys
# Expected: []
```

- [ ] **Step 5: Push to GitHub**

```bash
cd /home/mydelren/workspace/oc/context7-proxy
git push origin main
```

---

## Self-Review

**Completeness:**
- All 11 tasks have complete code — no placeholders, no "TODO", no "see implementation".
- Every task has explicit file paths and expected outputs.
- RequestLog model includes `Method` field, populated by proxy_service.

**Consistency:**
- All imports match between files (models, services, config).
- HTML JS fetch paths match Go router: `PUT /keys/:id`, `/manage/logs?limit=&offset=&status_code=`, `/manage/settings/master-key/reset`.
- JSON tags match JS field access: `latency_ms`, `method`, `endpoint`, `today_requests`.
- Dashboard fetches stats and timeseries from separate endpoints (`/manage/stats` + `/manage/stats/timeseries`).
- Dead `api.me()` removed.

**Verification:**
- Each task ends with a build or test step.
- Task 11 provides end-to-end verification: Docker build, health check, management API test.

**Risks:**
- Context7 upstream API path assumptions (`/v1/*`, `/api/*`) — may need adjustment if Context7 changes its route structure.
- Tailwind CDN and Chart.js CDN are loaded at runtime — no offline support without self-hosting.

---

## Execution Options

This plan can be executed in two ways:

### Option A: Subagent-Driven (Recommended)

Use `superpowers:subagent-driven-development` — each task is dispatched to a subagent that writes code, verifies, and commits. Best for parallelism and isolation.

### Option B: Inline Execution

Use `superpowers:executing-plans` — implement tasks sequentially in the current session. Each task: write files, run verification step, commit.

To start: ask the agent to **"Execute the Context7 proxy plan"** or **"Implement Task 1"**.
