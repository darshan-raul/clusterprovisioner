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

func TestRecordAndListHistory(t *testing.T) {
	db := openTestDB(t)
	s := New(db)
	ctx := context.Background()

	_ = s.EnsureUser(ctx, User{ID: "u-1", Username: "alice"})
	_ = s.EnsureUser(ctx, User{ID: "u-2", Username: "bob"})

	_ = s.CreateCluster(ctx, Cluster{ID: "cl-1", UserID: "u-1", Name: "c1", Context: "ctx1"}, ClusterCreds{})
	_ = s.CreateCluster(ctx, Cluster{ID: "cl-2", UserID: "u-1", Name: "c2", Context: "ctx2"}, ClusterCreds{})
	_ = s.CreateCluster(ctx, Cluster{ID: "cl-3", UserID: "u-2", Name: "c3", Context: "ctx3"}, ClusterCreds{})

	// Record actions
	a1 := ActionHistory{
		ID:         "act-1",
		UserID:     "u-1",
		ClusterID:  "cl-1",
		ActionType: "delete_pod",
		Target:     "default/nginx",
		Status:     "success",
		ClientType: "tui",
	}
	a2 := ActionHistory{
		ID:         "act-2",
		UserID:     "u-1",
		ClusterID:  "cl-2",
		ActionType: "apply_manifest",
		Target:     "1 resource(s)",
		Status:     "success",
		ClientType: "tui_agent",
	}
	a3 := ActionHistory{
		ID:         "act-3",
		UserID:     "u-2",
		ClusterID:  "cl-3",
		ActionType: "exec_command",
		Target:     "default/pod",
		Status:     "success",
		ClientType: "web",
	}

	if err := s.RecordAction(ctx, a1); err != nil {
		t.Fatalf("RecordAction 1: %v", err)
	}
	if err := s.RecordAction(ctx, a2); err != nil {
		t.Fatalf("RecordAction 2: %v", err)
	}
	if err := s.RecordAction(ctx, a3); err != nil {
		t.Fatalf("RecordAction 3: %v", err)
	}

	// List all history for u-1
	hist, err := s.ListHistory(ctx, "u-1", "", 10)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("expected 2 items for u-1, got %d", len(hist))
	}

	// List history for u-1 filtered by cl-1
	histCl1, err := s.ListHistory(ctx, "u-1", "cl-1", 10)
	if err != nil {
		t.Fatalf("ListHistory cl-1: %v", err)
	}
	if len(histCl1) != 1 || histCl1[0].ID != "act-1" {
		t.Errorf("expected act-1, got %+v", histCl1)
	}

	// Deleting cl-1 should cascade delete act-1
	if err := s.DeleteCluster(ctx, "u-1", "cl-1"); err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}
	histAfterDelete, err := s.ListHistory(ctx, "u-1", "cl-1", 10)
	if err != nil {
		t.Fatalf("ListHistory after delete: %v", err)
	}
	if len(histAfterDelete) != 0 {
		t.Errorf("expected 0 items after cascade delete, got %d", len(histAfterDelete))
	}
}

// _ keeps the sql import referenced in case the test file is
// refactored later (sql is referenced by the package's ErrNotFound).
var _ = sql.ErrNoRows
