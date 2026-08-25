package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate runs every embedded SQL file in lexical order. We don't
// track applied migrations yet (Phase 1 starts empty per kind dev
// cluster). Phase 2 will add a `schema_migrations` table.
func Migrate(ctx context.Context, db *sqlx.DB) error {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("Migrate: read dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := fs.ReadFile(migrationFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("Migrate: read %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(raw)); err != nil {
			return fmt.Errorf("Migrate: apply %s: %w", name, err)
		}
	}
	return nil
}
