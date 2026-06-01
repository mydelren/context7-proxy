// main.go
package main

import (
	"context"
	"embed"
	"log"
	"time"

	"github.com/mydelren/context7-proxy/internal/config"
	"github.com/mydelren/context7-proxy/internal/db"
	"github.com/mydelren/context7-proxy/internal/httpserver"
	"github.com/mydelren/context7-proxy/internal/services"
)

//go:embed static
var staticFS embed.FS

func main() {
	cfg := config.FromEnv()
	log.Printf("Context7 Proxy starting on %s", cfg.ListenAddr)

	gormDB, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	log.Println("Database initialized")

	// Load legacy master key for backward compatibility
	mk := services.NewMasterKeyService(gormDB, cfg.MasterKey)
	if err := mk.LoadOrCreate(context.Background()); err != nil {
		log.Fatalf("Master key init failed: %v", err)
	}

	// Auth service with legacy key fallback
	auth := services.NewAuthService(gormDB, mk.Get())
	if err := auth.InitAdmin(context.Background()); err != nil {
		log.Fatalf("Admin init failed: %v", err)
	}

	keys := services.NewKeyService(gormDB, cfg.CooldownSeconds)

	// Monthly used_count reset — check daily, reset on the 1st of each month (once)
	go func() {
		var lastResetMonth string
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			if now.Day() == 1 {
				month := now.Format("2006-01")
				if month != lastResetMonth {
					keys.ResetMonthlyUsage(context.Background())
					lastResetMonth = month
				}
			}
		}
	}()

	logs := services.NewLogService(gormDB)
	stats := services.NewStatsService(gormDB)

	// Daily log cleanup — delete logs older than 30 days
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		// Run once on startup, then every 24h
		logs.Cleanup(context.Background(), 30)
		for range ticker.C {
			logs.Cleanup(context.Background(), 30)
		}
	}()
	proxy := services.NewProxyService(cfg.Context7BaseURL, cfg.UpstreamTimeout, keys, logs)

	handler := httpserver.NewRouter(httpserver.Deps{
		Auth:     auth,
		Keys:     keys,
		Logs:     logs,
		Stats:    stats,
		Proxy:    proxy,
		StaticFS: staticFS,
	})

	httpserver.Run(cfg.ListenAddr, handler)
}
