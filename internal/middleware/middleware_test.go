package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/session"
)

func TestRequireAuthRedirectsMissingSession(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{CookieName: "sid"}}
	store := session.NewInMemoryStore()
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}), cfg, store)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/login" {
		t.Fatalf("Location = %q, want /login", location)
	}
}

func TestRequireAuthAddsUpstreamHeaders(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{CookieName: "sid"}}
	store := session.NewInMemoryStore()
	sess, err := store.Create("admin", time.Hour)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User"); got != "admin" {
			t.Fatalf("X-User = %q, want admin", got)
		}
		if got := r.Header.Get("X-Authenticated"); got != "true" {
			t.Fatalf("X-Authenticated = %q, want true", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}), cfg, store)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sid", Value: sess.ID})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
