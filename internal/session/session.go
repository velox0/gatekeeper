package session

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Session struct {
	ID        string
	User      string
	CreatedAt time.Time
	ExpiresAt time.Time
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
