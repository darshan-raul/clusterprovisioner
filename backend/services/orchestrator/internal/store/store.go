// Package store is the orchestrator's Postgres-backed data layer.
//
// Phase 1 supports two tables: users (Keycloak-subject cache) and
// clusters (one row per k8s cluster a user has registered, plus a
// pointer to the kubeconfig the MCP k8s server should use).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Cluster is one row of the clusters table joined with its credentials.
type Cluster struct {
	ID             string    `db:"id"             json:"id"`
	UserID         string    `db:"user_id"        json:"user_id"`
	Name           string    `db:"name"           json:"name"`
	Context        string    `db:"context"        json:"context"`
	CreatedAt      time.Time `db:"created_at"     json:"created_at"`
	KubeconfigPath string    `db:"kubeconfig_path" json:"kubeconfig_path"`
}

// User is the cached Keycloak subject.
type User struct {
	ID        string    `db:"id"         json:"id"`
	Username  string    `db:"username"   json:"username"`
	Email     *string   `db:"email"      json:"email,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Store is the orchestrator's data access layer.
type Store struct {
	db *sqlx.DB
}

// New wraps an existing sqlx.DB. The caller owns the connection
// pool lifecycle (closing it on shutdown).
func New(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// ErrNotFound is returned when a lookup misses.
var ErrNotFound = errors.New("store: not found")

// EnsureUser upserts the user record. Phase 1 keeps this minimal —
// we just cache the Keycloak subject so the foreign key from clusters
// is satisfied. Full user sync from Keycloak lands in Phase 2.
func (s *Store) EnsureUser(ctx context.Context, u User) error {
	if u.ID == "" || u.Username == "" {
		return fmt.Errorf("EnsureUser: id and username are required")
	}
	const q = `
		INSERT INTO users (id, username, email)
		VALUES (:id, :username, :email)
		ON CONFLICT (id) DO UPDATE
		SET username = EXCLUDED.username,
		    email    = EXCLUDED.email
	`
	if _, err := s.db.NamedExecContext(ctx, q, u); err != nil {
		return fmt.Errorf("EnsureUser: %w", err)
	}
	return nil
}

// ListClusters returns the user's registered clusters, ordered by
// creation time.
func (s *Store) ListClusters(ctx context.Context, userID string) ([]Cluster, error) {
	const q = `
		SELECT c.id, c.user_id, c.name, c.context, c.created_at,
		       COALESCE(cc.kubeconfig_path, '') AS kubeconfig_path
		FROM clusters c
		LEFT JOIN cluster_creds cc ON cc.cluster_id = c.id
		WHERE c.user_id = $1
		ORDER BY c.created_at
	`
	var out []Cluster
	if err := s.db.SelectContext(ctx, &out, q, userID); err != nil {
		return nil, fmt.Errorf("ListClusters: %w", err)
	}
	return out, nil
}

// GetCluster returns the cluster with the given ID if it belongs to
// the given user. ErrNotFound otherwise.
func (s *Store) GetCluster(ctx context.Context, userID, clusterID string) (*Cluster, error) {
	const q = `
		SELECT c.id, c.user_id, c.name, c.context, c.created_at,
		       COALESCE(cc.kubeconfig_path, '') AS kubeconfig_path
		FROM clusters c
		LEFT JOIN cluster_creds cc ON cc.cluster_id = c.id
		WHERE c.id = $1 AND c.user_id = $2
	`
	var c Cluster
	if err := s.db.GetContext(ctx, &c, q, clusterID, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("GetCluster: %w", err)
	}
	return &c, nil
}

// CreateCluster registers a new cluster for a user. Phase 1's
// "create cluster" flow is just the seeded mock cluster, but the
// method is here for the upcoming Phase 4 web form.
func (s *Store) CreateCluster(ctx context.Context, c Cluster, kubeconfigPath string) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("CreateCluster: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	const insertCluster = `
		INSERT INTO clusters (id, user_id, name, context)
		VALUES (:id, :user_id, :name, :context)
	`
	if _, err := tx.NamedExecContext(ctx, insertCluster, c); err != nil {
		return fmt.Errorf("CreateCluster insert cluster: %w", err)
	}
	const insertCreds = `
		INSERT INTO cluster_creds (cluster_id, kubeconfig_path)
		VALUES ($1, $2)
	`
	if _, err := tx.ExecContext(ctx, insertCreds, c.ID, kubeconfigPath); err != nil {
		return fmt.Errorf("CreateCluster insert creds: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("CreateCluster commit: %w", err)
	}
	return nil
}

// sqlxNoRows is unused now — keep an alias for any future caller
// that wants to compare without importing "database/sql".
var sqlxNoRows = sql.ErrNoRows
