// Command orchestrator is the Strata REST API service.
//
// It is the only thing the TUI talks to (besides Keycloak for the
// device-code OIDC dance). It validates JWTs, looks up cluster
// metadata, and proxies to MCP servers in the same cluster.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"

	"github.com/strata/orchestrator/internal/api"
	"github.com/strata/orchestrator/internal/config"
	"github.com/strata/orchestrator/internal/store"
	"github.com/strata/shared/pkg/jwks"
	"github.com/strata/shared/pkg/mcp"
)

func main() {
	log := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).
		With().Timestamp().Logger()
	zerolog.TimeFieldFormat = time.RFC3339

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err == nil {
		log = log.Level(level)
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("postgres connect")
	}
	defer db.Close()

	if err := store.Migrate(context.Background(), db); err != nil {
		log.Fatal().Err(err).Msg("migrate")
	}
	st := store.New(db)

	jwksValidator := jwks.New(jwks.Config{
		JWKSURL:  cfg.JWKSURL(),
		Issuer:   cfg.KeycloakAcceptedIssuers,
		Audience: cfg.JWTAudience,
	})

	mcpClient := mcp.NewClient(cfg.MCPK8sURL)

	srv := api.New(cfg, log, jwksValidator, st, mcpClient)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	idleDone := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutdown signal received")
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error().Err(err).Msg("http shutdown")
		}
		close(idleDone)
	}()

	log.Info().Str("addr", cfg.HTTPAddr).Msg("orchestrator listening")
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msg("listen")
	}
	<-idleDone
	log.Info().Msg("orchestrator stopped")
}

// openDB connects to Postgres with sqlx and pings.
//
// We use lib/pq here because CNPG exposes a stock Postgres wire
// protocol. pgx works too but lib/pq's registration as a
// database/sql driver is the most idiomatic for sqlx users.
func openDB(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}
