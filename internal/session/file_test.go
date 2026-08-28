package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStorePersistsSessionsAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore returned error: %v", err)
	}
	sess, err := store.Create("admin", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	reloaded, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reload returned error: %v", err)
	}
	got, ok := reloaded.Get(sess.ID)
	if !ok || got.User != "admin" {
		t.Fatalf("reloaded session = %+v, %v", got, ok)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestFileStoreDeletePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore returned error: %v", err)
	}
	sess, err := store.Create("admin", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	store.Delete(sess.ID)

	reloaded, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reload returned error: %v", err)
	}
	if _, ok := reloaded.Get(sess.ID); ok {
		t.Fatal("deleted session was restored")
	}
}

func TestFileStoreDropsExpiredSessionsOnLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("NewFileStore returned error: %v", err)
	}
	if _, err := store.Create("admin", -time.Second); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	reloaded, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reload returned error: %v", err)
	}
	if reloaded.Reap() != 0 {
		t.Fatal("expired session remained after load")
	}
}

func TestNewStoreFromEnvIsolatesNamespaces(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvDir, dir)
	first, err := NewStoreFromEnv(":8080\x00app.local")
	if err != nil {
		t.Fatalf("first store: %v", err)
	}
	second, err := NewStoreFromEnv(":8080\x00docs.local")
	if err != nil {
		t.Fatalf("second store: %v", err)
	}
	sess, err := first.Create("admin", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, ok := second.Get(sess.ID); ok {
		t.Fatal("session leaked into another virtual host store")
	}

	files, err := filepath.Glob(filepath.Join(dir, "sessions-*.json"))
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("session files = %d, want 1 written file", len(files))
	}
}
