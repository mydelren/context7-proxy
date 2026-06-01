// main.go
package main

import (
	"context"
	"embed"
	"log"

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

	mk := services.NewMasterKeyService(gormDB, cfg.MasterKey)
	if err := mk.LoadOrCreate(context.Background()); err != nil {
		log.Fatalf("Master key init failed: %v", err)
	}
	log.Printf("Master key: %s", mk.Get())

	keys := services.NewKeyService(gormDB, cfg.CooldownSeconds)
	logs := services.NewLogService(gormDB)
	stats := services.NewStatsService(gormDB)
	proxy := services.NewProxyService(cfg.Context7BaseURL, cfg.UpstreamTimeout, keys, logs)

	handler := httpserver.NewRouter(httpserver.Deps{
		MasterKey: mk,
		Keys:      keys,
		Logs:      logs,
		Stats:     stats,
		Proxy:     proxy,
		StaticFS:  staticFS,
	})

	httpserver.Run(cfg.ListenAddr, handler)
}
