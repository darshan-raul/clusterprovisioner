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
	ID                  string    `db:"id"                   json:"id"`
	UserID              string    `db:"user_id"              json:"user_id"`
	Name                string    `db:"name"                 json:"name"`
	Context             string    `db:"context"              json:"context"`
	CreatedAt           time.Time `db:"created_at"           json:"created_at"`
	KubeconfigPath      string    `db:"kubeconfig_path"      json:"kubeconfig_path,omitempty"`
	EncryptedKubeconfig string    `db:"encrypted_kubeconfig" json:"-"`
	DEKCiphertext       string    `db:"dek_ciphertext"       json:"-"`
}

// ClusterCreds holds credential data associated with a cluster.
type ClusterCreds struct {
	KubeconfigPath      string
	EncryptedKubeconfig string
	DEKCiphertext       string
}

// User is the cached Keycloak subject.
type User struct {
	ID        string    `db:"id"         json:"id"`
	Username  string    `db:"username"   json:"username"`
	Email     *string   `db:"email"      json:"email,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ActionHistory represents an audited action taken on a cluster.
type ActionHistory struct {
	ID         string    `db:"id"          json:"id"`
	UserID     string    `db:"user_id"     json:"user_id"`
	ClusterID  string    `db:"cluster_id"  json:"cluster_id"`
	ActionType string    `db:"action_type" json:"action_type"`
	Target     string    `db:"target"      json:"target"`
	Status     string    `db:"status"      json:"status"`
	Details    string    `db:"details"     json:"details"`
	ClientType string    `db:"client_type" json:"client_type"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
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
		       COALESCE(cc.kubeconfig_path, '') AS kubeconfig_path,
		       COALESCE(cc.encrypted_kubeconfig, '') AS encrypted_kubeconfig,
		       COALESCE(cc.dek_ciphertext, '') AS dek_ciphertext
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
		       COALESCE(cc.kubeconfig_path, '') AS kubeconfig_path,
		       COALESCE(cc.encrypted_kubeconfig, '') AS encrypted_kubeconfig,
		       COALESCE(cc.dek_ciphertext, '') AS dek_ciphertext
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

// CreateCluster registers a new cluster for a user.
func (s *Store) CreateCluster(ctx context.Context, c Cluster, creds ClusterCreds) error {
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
		INSERT INTO cluster_creds (cluster_id, kubeconfig_path, encrypted_kubeconfig, dek_ciphertext)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := tx.ExecContext(ctx, insertCreds, c.ID, creds.KubeconfigPath, creds.EncryptedKubeconfig, creds.DEKCiphertext); err != nil {
		return fmt.Errorf("CreateCluster insert creds: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("CreateCluster commit: %w", err)
	}
	return nil
}

// DeleteCluster removes a cluster and its associated credentials for the given user.
// ErrNotFound is returned if no matching cluster exists.
func (s *Store) DeleteCluster(ctx context.Context, userID, clusterID string) error {
	const q = `DELETE FROM clusters WHERE id = $1 AND user_id = $2`
	res, err := s.db.ExecContext(ctx, q, clusterID, userID)
	if err != nil {
		return fmt.Errorf("DeleteCluster: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteCluster rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RecordAction logs an action into the audit trail.
func (s *Store) RecordAction(ctx context.Context, a ActionHistory) error {
	const q = `
		INSERT INTO action_history (id, user_id, cluster_id, action_type, target, status, details, client_type, created_at)
		VALUES (:id, :user_id, :cluster_id, :action_type, :target, :status, :details, :client_type, :created_at)
	`
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
	if _, err := s.db.NamedExecContext(ctx, q, a); err != nil {
		return fmt.Errorf("RecordAction: %w", err)
	}
	return nil
}

// ListHistory returns the user's action history, optionally filtered by clusterID.
func (s *Store) ListHistory(ctx context.Context, userID, clusterID string, limit int) ([]ActionHistory, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []ActionHistory
	if clusterID != "" {
		const q = `
			SELECT id, user_id, cluster_id, action_type, target, status, details, client_type, created_at
			FROM action_history
			WHERE user_id = $1 AND cluster_id = $2
			ORDER BY created_at DESC
			LIMIT $3
		`
		if err := s.db.SelectContext(ctx, &out, q, userID, clusterID, limit); err != nil {
			return nil, fmt.Errorf("ListHistory: %w", err)
		}
	} else {
		const q = `
			SELECT id, user_id, cluster_id, action_type, target, status, details, client_type, created_at
			FROM action_history
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT $2
		`
		if err := s.db.SelectContext(ctx, &out, q, userID, limit); err != nil {
			return nil, fmt.Errorf("ListHistory: %w", err)
		}
	}
	return out, nil
}

// sqlxNoRows is unused now — keep an alias for any future caller
// that wants to compare without importing "database/sql".
var sqlxNoRows = sql.ErrNoRows
