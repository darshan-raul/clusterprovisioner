// Package config loads environment-based configuration for Strata services.
//
// Each Strata service (orchestrator, retriever, rag-indexer, MCP servers)
// reads its config from environment variables at startup. We don't use
// a config file in Phase 1 — env vars only. Phase 8+ adds a config file
// for prod, but env vars stay the default for dev/kind.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Base is the configuration shared by all backend services. Embed it
// in service-specific configs.
type Base struct {
	LogLevel        string
	HTTPAddr        string
	ShutdownTimeout time.Duration
}

// LoadBase reads the base config from env vars.
func LoadBase() (Base, error) {
	cfg := Base{
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Base{}, fmt.Errorf("invalid LOG_LEVEL %q (want debug/info/warn/error)", cfg.LogLevel)
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
