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
