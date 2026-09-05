package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/strata/rag-indexer/internal/config"
	"github.com/strata/rag-indexer/internal/indexer"
)

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()
	cfg := config.Load()

	log.Info().
		Str("retriever_url", cfg.RetrieverURL).
		Str("docs_dir", cfg.DocsDir).
		Bool("once", cfg.Once).
		Int("interval_seconds", cfg.IntervalSeconds).
		Msg("starting rag-indexer service")

	idx := indexer.New(cfg.RetrieverURL, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runSync := func() {
		start := time.Now()
		log.Info().Msg("starting RAG indexing sync cycle")

		// Index documentation for default/system user
		docCount, err := idx.IndexDocumentation(ctx, "system", cfg.DocsDir)
		if err != nil {
			log.Warn().Err(err).Msg("documentation indexing error")
		} else {
			log.Info().Int("docs_indexed", docCount).Msg("documentation indexing complete")
		}

		log.Info().Dur("duration", time.Since(start)).Msg("indexing sync cycle finished")
	}

	runSync()

	if cfg.Once {
		log.Info().Msg("single-shot sync completed (--once flag set), exiting")
		return
	}

	ticker := time.NewTicker(time.Duration(cfg.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigCh:
			log.Info().Msg("rag-indexer shutting down")
			return
		case <-ticker.C:
			runSync()
		}
	}
}
