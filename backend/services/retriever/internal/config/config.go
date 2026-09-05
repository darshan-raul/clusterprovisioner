package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port            int
	QdrantURL       string
	EmbeddingURL    string
	EmbeddingModel  string
	EmbeddingAPIKey string
	JWTAudience     string
	JWKSEndpoint    string
	DevMode         bool
}

func Load() Config {
	port := 8082
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}
	devMode := true
	if os.Getenv("DEV_MODE") == "false" || (os.Getenv("QDRANT_URL") != "" && os.Getenv("DEV_MODE") != "true") {
		devMode = false
	}
	return Config{
		Port:            port,
		QdrantURL:       getenvDefault("QDRANT_URL", "http://localhost:6333"),
		EmbeddingURL:    getenvDefault("EMBEDDING_URL", "http://localhost:4000/v1/embeddings"),
		EmbeddingModel:  getenvDefault("EMBEDDING_MODEL", "amazon.titan-embed-text-v2:0"),
		EmbeddingAPIKey: os.Getenv("EMBEDDING_API_KEY"),
		JWTAudience:     getenvDefault("JWT_AUDIENCE", "strata-tui"),
		JWKSEndpoint:    getenvDefault("JWKS_ENDPOINT", "http://localhost:8081/realms/strata/protocol/openid-connect/certs"),
		DevMode:         devMode,
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
