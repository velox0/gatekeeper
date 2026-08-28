package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EnvDir is the environment variable used to enable persistent sessions.
const EnvDir = "GATEKEEPER_SESSION_DIR"

// FileStore persists sessions for one virtual host in an atomic JSON file.
type FileStore struct {
	mu       sync.RWMutex
	path     string
	sessions map[string]*Session
}

// NewStoreFromEnv returns an in-memory store unless EnvDir is configured.
func NewStoreFromEnv(namespace string) (Store, error) {
	dir := os.Getenv(EnvDir)
	if dir == "" {
		return NewInMemoryStore(), nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create session directory %q: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure session directory %q: %w", dir, err)
	}
	sum := sha256.Sum256([]byte(namespace))
	name := "sessions-" + hex.EncodeToString(sum[:8]) + ".json"
	return NewFileStore(filepath.Join(dir, name))
}

// NewFileStore loads or creates a persistent session store at path.
func NewFileStore(path string) (*FileStore, error) {
	s := &FileStore{path: path, sessions: make(map[string]*Session)}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read sessions %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("secure sessions %q: %w", path, err)
	}
	if len(data) != 0 {
		if err := json.Unmarshal(data, &s.sessions); err != nil {
			return nil, fmt.Errorf("decode sessions %q: %w", path, err)
		}
	}
	s.Reap()
	return s, nil
}

// Create adds and persists a new session.
func (s *FileStore) Create(user string, ttl time.Duration) (*Session, error) {
	id, err := genSessionID(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	sess := &Session{ID: id, User: user, CreatedAt: now, ExpiresAt: now.Add(ttl)}

	s.mu.Lock()
	s.sessions[id] = sess
	if err := s.persistLocked(); err != nil {
		delete(s.sessions, id)
		s.mu.Unlock()
		return nil, err
	}
	s.mu.Unlock()
	copy := *sess
	return &copy, nil
}

// Get retrieves a non-expired session.
func (s *FileStore) Get(id string) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	if !ok || sess.ExpiresAt.Before(time.Now()) {
		s.mu.RUnlock()
		if ok {
			s.Delete(id)
		}
		return nil, false
	}
	copy := *sess
	s.mu.RUnlock()
	return &copy, true
}

// Delete removes and persists a session.
func (s *FileStore) Delete(id string) {
	s.mu.Lock()
	previous, ok := s.sessions[id]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.sessions, id)
	if err := s.persistLocked(); err != nil {
		s.sessions[id] = previous
		log.Printf("failed to persist session deletion: %v", err)
	}
	s.mu.Unlock()
}

// Reap removes expired sessions and persists the updated store.
func (s *FileStore) Reap() int {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := make(map[string]*Session)
	for id, sess := range s.sessions {
		if sess.ExpiresAt.Before(now) {
			removed[id] = sess
			delete(s.sessions, id)
		}
	}
	if len(removed) == 0 {
		return 0
	}
	if err := s.persistLocked(); err != nil {
		for id, sess := range removed {
			s.sessions[id] = sess
		}
		log.Printf("failed to persist expired session cleanup: %v", err)
		return 0
	}
	return len(removed)
}

// StartReaper launches periodic cleanup until ctx is cancelled.
func (s *FileStore) StartReaper(ctx context.Context, interval time.Duration) {
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

func (s *FileStore) persistLocked() error {
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, ".gatekeeper-sessions-*")
	if err != nil {
		return fmt.Errorf("create temporary session file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	if err := encoder.Encode(s.sessions); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode sessions: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("secure temporary session file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync sessions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary session file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace session file: %w", err)
	}
	return nil
}
