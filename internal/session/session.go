package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"log"
	"sync"
	"time"
)

type Session struct {
	ID        string
	User      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Store is the session storage contract used by authentication middleware.
type Store interface {
	Create(user string, ttl time.Duration) (*Session, error)
	Get(id string) (*Session, bool)
	Delete(id string)
	Reap() int
	StartReaper(ctx context.Context, interval time.Duration)
}

type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{sessions: make(map[string]*Session)}
}

func genSessionID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *InMemoryStore) Create(user string, ttl time.Duration) (*Session, error) {
	id, err := genSessionID(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{
		ID:        id,
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess, nil
}

func (s *InMemoryStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	if !ok {
		s.mu.RUnlock()
		return nil, false
	}
	if sess.ExpiresAt.Before(time.Now()) {
		s.mu.RUnlock()
		s.Delete(id)
		return nil, false
	}
	s.mu.RUnlock()
	return sess, true
}

func (s *InMemoryStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// Reap removes all expired sessions from the store and returns
// the number of sessions that were removed.
func (s *InMemoryStore) Reap() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	reaped := 0
	for id, sess := range s.sessions {
		if sess.ExpiresAt.Before(now) {
			delete(s.sessions, id)
			reaped++
		}
	}
	return reaped
}

// StartReaper launches a background goroutine that periodically removes
// expired sessions. It stops when the provided context is cancelled.
func (s *InMemoryStore) StartReaper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n := s.Reap(); n > 0 {
					log.Printf("session reaper: removed %d expired session(s)", n)
				}
			}
		}
	}()
}
