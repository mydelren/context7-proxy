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
