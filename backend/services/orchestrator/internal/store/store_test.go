package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// openTestDB opens an in-memory sqlite-compatible store and runs
// the production migrations against it.
//
// Note: sqlite doesn't support every PostgreSQL type/feature, so
// some queries may need adjusting if they use Postgres-only syntax.
// The store's CRUD path uses parameterized queries that work on
// both engines.
func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := filepath.ToSlash(t.TempDir()) + "/test.db?cache=shared&_pragma=foreign_keys(1)"
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.Ping(); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMigrate_Idempotent(t *testing.T) {
	db := openTestDB(t)
	if err := Migrate(context.Background(), db); err != nil {
		t.Errorf("second migrate: %v", err)
	}
}

func TestEnsureUser_RoundTrip(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	// Update should change username/email without dropping the row.
	email := "alice@example.com"
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice2", Email: &email}); err != nil {
		t.Fatal(err)
	}
	row := db.QueryRowxContext(ctx, `SELECT username, email FROM users WHERE id = 'u-1'`)
	var got struct {
		Username string  `db:"username"`
		Email    *string `db:"email"`
	}
	if err := row.StructScan(&got); err != nil {
		t.Fatal(err)
	}
	if got.Username != "alice2" {
		t.Errorf("username = %q", got.Username)
	}
	if got.Email == nil || *got.Email != email {
		t.Errorf("email = %v", got.Email)
	}
}

func TestCreateAndGetCluster(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}

	c := Cluster{
		ID:      "cl-001",
		UserID:  "u-1",
		Name:    "demo",
		Context: "demo-context",
	}
	if err := s.CreateCluster(ctx, c, ClusterCreds{KubeconfigPath: "/etc/strata/kubeconfigs/cl-001"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCluster(ctx, "u-1", "cl-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "demo" {
		t.Errorf("name = %q", got.Name)
	}
	if got.KubeconfigPath != "/etc/strata/kubeconfigs/cl-001" {
		t.Errorf("path = %q", got.KubeconfigPath)
	}
}

func TestCreateAndGetCluster_Encrypted(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}

	c := Cluster{
		ID:      "cl-enc-001",
		UserID:  "u-1",
		Name:    "prod-cluster",
		Context: "prod-ctx",
	}
	creds := ClusterCreds{
		EncryptedKubeconfig: "base64-enc-kubeconfig",
		DEKCiphertext:       "kms-dek-123",
	}
	if err := s.CreateCluster(ctx, c, creds); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetCluster(ctx, "u-1", "cl-enc-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.EncryptedKubeconfig != "base64-enc-kubeconfig" {
		t.Errorf("encrypted = %q", got.EncryptedKubeconfig)
	}
	if got.DEKCiphertext != "kms-dek-123" {
		t.Errorf("dek = %q", got.DEKCiphertext)
	}
}

func TestDeleteCluster(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	c := Cluster{ID: "cl-del", UserID: "u-1", Name: "to-delete", Context: "ctx"}
	if err := s.CreateCluster(ctx, c, ClusterCreds{EncryptedKubeconfig: "enc"}); err != nil {
		t.Fatal(err)
	}

	// u-2 cannot delete u-1's cluster
	if err := s.DeleteCluster(ctx, "u-2", "cl-del"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}

	// u-1 successfully deletes
	if err := s.DeleteCluster(ctx, "u-1", "cl-del"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// now it's gone
	if _, err := s.GetCluster(ctx, "u-1", "cl-del"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}

	// cluster_creds row should also be gone (cascade)
	var count int
	if err := db.GetContext(ctx, &count, `SELECT count(*) FROM cluster_creds WHERE cluster_id = 'cl-del'`); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("cluster_creds count = %d, want 0 after cascade delete", count)
	}
}

func TestGetCluster_NotFound(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetCluster(ctx, "u-1", "missing")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetCluster_OtherUserForbidden(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureUser(ctx, User{ID: "u-2", Username: "bob"}); err != nil {
		t.Fatal(err)
	}
	c := Cluster{ID: "cl-001", UserID: "u-1", Name: "demo", Context: "demo-context"}
	if err := s.CreateCluster(ctx, c, ClusterCreds{KubeconfigPath: "/etc/strata/kubeconfigs/cl-001"}); err != nil {
		t.Fatal(err)
	}
	// u-2 must not see u-1's cluster.
	if _, err := s.GetCluster(ctx, "u-2", "cl-001"); err != ErrNotFound {
		t.Errorf("u-2 sees u-1's cluster: err = %v", err)
	}
}

func TestListClusters_OrderedByCreatedAt(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()
	if err := s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"cl-001", "cl-002", "cl-003"} {
		if err := s.CreateCluster(ctx, Cluster{ID: id, UserID: "u-1", Name: id, Context: "ctx"}, ClusterCreds{KubeconfigPath: "/p/" + id}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.ListClusters(ctx, "u-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d clusters", len(got))
	}
	if got[0].ID != "cl-001" || got[1].ID != "cl-002" || got[2].ID != "cl-003" {
		t.Errorf("ids = %v %v %v", got[0].ID, got[1].ID, got[2].ID)
	}
}

// TestEnsureUser_RequiresFields documents the validation contract.
func TestEnsureUser_RequiresFields(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	if err := s.EnsureUser(context.Background(), User{Username: "alice"}); err == nil {
		t.Error("expected error for missing id")
	}
	if err := s.EnsureUser(context.Background(), User{ID: "u-1"}); err == nil {
		t.Error("expected error for missing username")
	}
}

// _ keeps the sql import referenced in case the test file is
// refactored later (sql is referenced by the package's ErrNotFound).
var _ = sql.ErrNoRows
