// Package ratelimit provides a fixed-window, in-memory attempt limiter.
package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	count   int
	resetAt time.Time
}

// Limiter allows at most max attempts per key within each window.
type Limiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	entries   map[string]*entry
	nextPrune time.Time
	now       func() time.Time
}

// New returns a Limiter allowing max attempts per key per window.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		max:     max,
		window:  window,
		entries: make(map[string]*entry),
		now:     time.Now,
	}
}

// Reserve counts an attempt for key before it is carried out, so concurrent
// attempts cannot overshoot the limit. If the limit is reached it returns
// false and the time until key may try again.
func (l *Limiter) Reserve(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneLocked(now)
	e, ok := l.entries[key]
	if !ok || !now.Before(e.resetAt) {
		e = &entry{resetAt: now.Add(l.window)}
		l.entries[key] = e
	}
	if e.count >= l.max {
		return false, e.resetAt.Sub(now)
	}
	e.count++
	return true, 0
}

// Release returns one reserved attempt to key.
func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e, ok := l.entries[key]; ok && e.count > 0 {
		e.count--
	}
}

// pruneLocked drops expired entries at most once per window.
func (l *Limiter) pruneLocked(now time.Time) {
	if now.Before(l.nextPrune) {
		return
	}
	for key, e := range l.entries {
		if !now.Before(e.resetAt) {
			delete(l.entries, key)
		}
	}
	l.nextPrune = now.Add(l.window)
}
