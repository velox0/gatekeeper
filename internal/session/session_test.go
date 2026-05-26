package session

import (
	"testing"
	"time"
)

func TestStoreCreateGetDelete(t *testing.T) {
	store := NewInMemoryStore()

	sess, err := store.Create("admin", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if sess.ID == "" {
		t.Fatal("session ID was empty")
	}

	got, ok := store.Get(sess.ID)
	if !ok {
		t.Fatal("Get returned ok=false")
	}
	if got.User != "admin" {
		t.Fatalf("User = %q, want admin", got.User)
	}

	store.Delete(sess.ID)
	if _, ok := store.Get(sess.ID); ok {
		t.Fatal("Get returned ok=true after Delete")
	}
}

func TestStoreGetRemovesExpiredSession(t *testing.T) {
	store := NewInMemoryStore()

	sess, err := store.Create("admin", -time.Second)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, ok := store.Get(sess.ID); ok {
		t.Fatal("Get returned ok=true for expired session")
	}

	store.mu.RLock()
	_, exists := store.sessions[sess.ID]
	store.mu.RUnlock()
	if exists {
		t.Fatal("expired session was not removed")
	}
}
