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
