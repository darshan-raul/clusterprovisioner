package config

import (
	"os"
	"strconv"
)

type Config struct {
	RetrieverURL        string
	OrchestratorURL     string
	DocsDir             string
	IntervalSeconds     int
	Once                bool
	BootstrapAdminToken string
}

func Load() Config {
	interval := 60
	if s := os.Getenv("INDEX_INTERVAL_SECONDS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			interval = v
		}
	}
	once := os.Getenv("INDEX_ONCE") == "true"
	for _, arg := range os.Args[1:] {
		if arg == "--once" {
			once = true
		}
	}
	return Config{
		RetrieverURL:        getenvDefault("RETRIEVER_URL", "http://localhost:8082"),
		OrchestratorURL:     getenvDefault("ORCHESTRATOR_URL", "http://localhost:8080"),
		DocsDir:             getenvDefault("DOCS_DIR", "docs"),
		IntervalSeconds:     interval,
		Once:                once,
		BootstrapAdminToken: os.Getenv("BOOTSTRAP_ADMIN_TOKEN"),
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
