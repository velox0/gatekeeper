package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func newTestLimiter(max int, window time.Duration) (*Limiter, *time.Time) {
	now := time.Unix(1000, 0)
	l := New(max, window)
	l.now = func() time.Time { return now }
	return l, &now
}

func TestReserveBlocksAtLimitAndReportsRetry(t *testing.T) {
	l, now := newTestLimiter(2, time.Minute)

	for i := 0; i < 2; i++ {
		if ok, _ := l.Reserve("key"); !ok {
			t.Fatalf("attempt %d was blocked", i+1)
		}
	}
	*now = now.Add(20 * time.Second)
	ok, retry := l.Reserve("key")
	if ok {
		t.Fatal("attempt over the limit was allowed")
	}
	if retry != 40*time.Second {
		t.Fatalf("retry = %v, want 40s", retry)
	}
}

func TestReserveTracksKeysIndependently(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)

	if ok, _ := l.Reserve("a"); !ok {
		t.Fatal("first key was blocked")
	}
	if ok, _ := l.Reserve("b"); !ok {
		t.Fatal("second key was blocked by the first")
	}
}

func TestReserveAllowsAgainAfterWindow(t *testing.T) {
	l, now := newTestLimiter(1, time.Minute)

	l.Reserve("key")
	if ok, _ := l.Reserve("key"); ok {
		t.Fatal("attempt over the limit was allowed")
	}
	*now = now.Add(time.Minute)
	if ok, _ := l.Reserve("key"); !ok {
		t.Fatal("attempt after the window was blocked")
	}
}

func TestBlockedAttemptsDoNotExtendWindow(t *testing.T) {
	l, now := newTestLimiter(1, time.Minute)

	l.Reserve("key")
	*now = now.Add(59 * time.Second)
	l.Reserve("key")
	*now = now.Add(time.Second)
	if ok, _ := l.Reserve("key"); !ok {
		t.Fatal("blocked attempt extended the window")
	}
}

func TestReleaseReturnsOneAttempt(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)

	l.Reserve("key")
	l.Release("key")
	if ok, _ := l.Reserve("key"); !ok {
		t.Fatal("released attempt was not returned")
	}
	l.Release("key")
	l.Release("key")
	l.Release("unknown")
	l.Reserve("key")
	if ok, _ := l.Reserve("key"); ok {
		t.Fatal("extra releases raised the limit")
	}
}

func TestReservePrunesExpiredEntries(t *testing.T) {
	l, now := newTestLimiter(1, time.Minute)

	l.Reserve("old")
	*now = now.Add(time.Minute)
	l.Reserve("new")

	if _, ok := l.entries["old"]; ok {
		t.Fatal("expired entry was not pruned")
	}
	if len(l.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(l.entries))
	}
}

func TestReserveConcurrentAttemptsCannotOvershoot(t *testing.T) {
	l := New(5, time.Minute)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		allowed int
	)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.Reserve("key"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 5 {
		t.Fatalf("allowed = %d, want 5", allowed)
	}
}
