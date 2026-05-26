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

func TestReapRemovesExpiredSessions(t *testing.T) {
	store := NewInMemoryStore()

	// Create an expired session.
	_, err := store.Create("expired-user", -time.Second)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Create a valid session.
	valid, err := store.Create("valid-user", time.Hour)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	reaped := store.Reap()
	if reaped != 1 {
		t.Fatalf("Reap() = %d, want 1", reaped)
	}

	// Valid session should still exist.
	if _, ok := store.Get(valid.ID); !ok {
		t.Fatal("valid session should still exist after reap")
	}
}

func TestReapLeavesActiveSessions(t *testing.T) {
	store := NewInMemoryStore()

	_, err := store.Create("active-user", time.Hour)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}

	reaped := store.Reap()
	if reaped != 0 {
		t.Fatalf("Reap() = %d, want 0", reaped)
	}

	store.mu.RLock()
	count := len(store.sessions)
	store.mu.RUnlock()
	if count != 1 {
		t.Fatalf("sessions count = %d, want 1", count)
	}
}

func TestConcurrentCreateAndGet(t *testing.T) {
	store := NewInMemoryStore()
	const n = 100

	// Create sessions concurrently.
	done := make(chan *Session, n)
	for i := 0; i < n; i++ {
		go func() {
			sess, err := store.Create("user", time.Hour)
			if err != nil {
				t.Errorf("Create error: %v", err)
			}
			done <- sess
		}()
	}

	sessions := make([]*Session, 0, n)
	for i := 0; i < n; i++ {
		sessions = append(sessions, <-done)
	}

	// Get all sessions concurrently.
	for _, sess := range sessions {
		go func(id string) {
			if _, ok := store.Get(id); !ok {
				t.Errorf("Get(%q) returned ok=false", id)
			}
		}(sess.ID)
	}
}
