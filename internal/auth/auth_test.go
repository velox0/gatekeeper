package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/session"
)

func newTestHandler(t *testing.T) (*Handler, *session.InMemoryStore) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	store := session.NewInMemoryStore()
	rc := &config.ResolvedConfig{
		Auth: config.AuthConfig{
			CookieName: "sid",
			SessionTTL: time.Hour,
		},
		Users: []config.UserConfig{
			{
				Username:     "admin",
				PasswordHash: string(hash),
			},
		},
		Plugins: map[string]bool{},
	}
	handler, err := NewHandler(rc, store)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}

	return handler, store
}

func TestLoginGetRendersForm(t *testing.T) {
	handler, _ := newTestHandler(t)
	rec := httptest.NewRecorder()

	handler.LoginHandler(rec, httptest.NewRequest(http.MethodGet, "/login", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want text/html; charset=utf-8", got)
	}
	if !strings.Contains(rec.Body.String(), `action="/login"`) {
		t.Fatalf("login response did not include the login form")
	}
}

func TestLoginPostCreatesSessionCookie(t *testing.T) {
	handler, store := newTestHandler(t)
	form := url.Values{
		"username": {"admin"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.LoginHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/" {
		t.Fatalf("Location = %q, want /", location)
	}

	cookie := findCookie(t, rec.Result().Cookies(), "sid")
	if cookie.Value == "" {
		t.Fatal("session cookie value was empty")
	}
	if _, ok := store.Get(cookie.Value); !ok {
		t.Fatal("session cookie did not map to a stored session")
	}
}

func TestLoginPostRejectsInvalidPassword(t *testing.T) {
	handler, _ := newTestHandler(t)
	form := url.Values{
		"username": {"admin"},
		"password": {"wrong"},
	}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.LoginHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "/login") || !strings.Contains(location, "error=") {
		t.Fatalf("Location = %q, want redirect to /login with error param", location)
	}
	if cookie := findOptionalCookie(rec.Result().Cookies(), "sid"); cookie != nil {
		t.Fatalf("unexpected session cookie: %v", cookie)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	cookie := findOptionalCookie(cookies, name)
	if cookie == nil {
		t.Fatalf("cookie %q not found", name)
	}
	return cookie
}

func findOptionalCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}
