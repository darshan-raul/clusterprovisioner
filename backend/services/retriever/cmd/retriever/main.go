package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/strata/retriever/internal/api"
	"github.com/strata/retriever/internal/config"
	"github.com/strata/retriever/internal/embedder"
	"github.com/strata/retriever/internal/vectorstore"
	"github.com/strata/shared/pkg/jwks"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cfg := config.Load()

	log.Info().
		Int("port", cfg.Port).
		Bool("dev_mode", cfg.DevMode).
		Str("embedding_model", cfg.EmbeddingModel).
		Msg("starting retriever service")

	var emb embedder.Embedder
	var st vectorstore.Store

	if cfg.DevMode {
		log.Info().Msg("using in-memory vector store and deterministic mock embedder")
		emb = embedder.NewMockEmbedder(128)
		st = vectorstore.NewMemoryStore()
	} else {
		log.Info().Str("url", cfg.EmbeddingURL).Msg("using OpenAI/LiteLLM embedder")
		emb = embedder.NewOpenAIEmbedder(cfg.EmbeddingURL, cfg.EmbeddingModel, cfg.EmbeddingAPIKey, 1536)
		log.Info().Str("qdrant_url", cfg.QdrantURL).Msg("using Qdrant vector store")
		st = vectorstore.NewQdrantStore(cfg.QdrantURL, 1536)
	}

	var jwksVal *jwks.Validator
	if cfg.JWKSEndpoint != "" {
		jwksVal = jwks.New(jwks.Config{
			JWKSURL:  cfg.JWKSEndpoint,
			Audience: cfg.JWTAudience,
		})
	}

	srv := api.New(cfg, log, jwksVal, emb, st)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      srv.Router(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failure")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	log.Info().Msg("retriever service stopped")
}
