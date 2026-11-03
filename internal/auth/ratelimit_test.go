package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

type loginAttempt struct {
	username string
	password string
	remote   string
	xff      string
}

func attemptLogin(handler *Handler, attempt loginAttempt) *httptest.ResponseRecorder {
	form := url.Values{"username": {attempt.username}, "password": {attempt.password}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = attempt.remote
	if attempt.xff != "" {
		req.Header.Set("X-Forwarded-For", attempt.xff)
	}
	rec := httptest.NewRecorder()
	handler.LoginHandler(rec, req)
	return rec
}

func TestLoginBlocksUsernameAfterFailedAttempts(t *testing.T) {
	handler, _ := newTestHandler(t)

	for i := 0; i < maxLoginAttemptsPerUser; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1234", i+1)
		rec := attemptLogin(handler, loginAttempt{username: "admin", password: "wrong", remote: remote})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("attempt %d: status = %d, want %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	// The correct password is rejected too: nothing is checked once blocked.
	rec := attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: "198.51.100.1:1234"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter <= 0 {
		t.Fatalf("Retry-After = %q, want positive seconds", rec.Header().Get("Retry-After"))
	}
	if !strings.Contains(rec.Body.String(), "Too many login attempts") {
		t.Fatal("login page did not explain the block")
	}
	if cookie := findOptionalCookie(rec.Result().Cookies(), "sid"); cookie != nil {
		t.Fatalf("unexpected session cookie: %v", cookie)
	}
}

func TestLoginBlocksUnknownUsernamesLikeRealOnes(t *testing.T) {
	handler, _ := newTestHandler(t)

	for i := 0; i < maxLoginAttemptsPerUser; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1234", i+1)
		attemptLogin(handler, loginAttempt{username: "nobody", password: "wrong", remote: remote})
	}
	rec := attemptLogin(handler, loginAttempt{username: "nobody", password: "wrong", remote: "198.51.100.1:1234"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginBlocksIPAcrossUsernames(t *testing.T) {
	handler, _ := newTestHandler(t)
	const remote = "203.0.113.1:1234"

	for i := 0; i < maxLoginAttemptsPerIP; i++ {
		rec := attemptLogin(handler, loginAttempt{username: fmt.Sprintf("user%d", i), password: "wrong", remote: remote})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("attempt %d: status = %d, want %d", i+1, rec.Code, http.StatusSeeOther)
		}
	}

	rec := attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: remote})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}

	// A blocked IP must not use up the attempts of the username it targets.
	rec = attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: "198.51.100.1:1234"})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("other client: status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestLoginBlockedUsernameDoesNotCountAgainstIP(t *testing.T) {
	handler, _ := newTestHandler(t)

	for i := 0; i < maxLoginAttemptsPerUser; i++ {
		remote := fmt.Sprintf("203.0.113.%d:1234", i+1)
		attemptLogin(handler, loginAttempt{username: "admin", password: "wrong", remote: remote})
	}
	const remote = "198.51.100.1:1234"
	for i := 0; i < maxLoginAttemptsPerIP*2; i++ {
		attemptLogin(handler, loginAttempt{username: "admin", password: "wrong", remote: remote})
	}

	rec := attemptLogin(handler, loginAttempt{username: "other", password: "wrong", remote: remote})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
}

func TestLoginSuccessDoesNotCountTowardsLimits(t *testing.T) {
	handler, _ := newTestHandler(t)
	const remote = "203.0.113.1:1234"

	for i := 0; i < maxLoginAttemptsPerIP*2; i++ {
		rec := attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: remote})
		if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/") {
			t.Fatalf("login %d: status = %d, Location = %q", i+1, rec.Code, rec.Header().Get("Location"))
		}
	}
}

func TestLoginOversizedPasswordDoesNotCountTowardsLimits(t *testing.T) {
	handler, _ := newTestHandler(t)
	const remote = "203.0.113.1:1234"
	oversized := strings.Repeat("x", maxBcryptPasswordLen+1)

	for i := 0; i < maxLoginAttemptsPerIP*2; i++ {
		attemptLogin(handler, loginAttempt{username: "admin", password: oversized, remote: remote})
	}
	rec := attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: remote})
	if findOptionalCookie(rec.Result().Cookies(), "sid") == nil {
		t.Fatalf("login was blocked: status = %d", rec.Code)
	}
}

func TestLoginIgnoresForwardedForFromUntrustedPeer(t *testing.T) {
	handler, _ := newTestHandler(t)
	const remote = "203.0.113.1:1234"

	for i := 0; i < maxLoginAttemptsPerIP; i++ {
		attemptLogin(handler, loginAttempt{
			username: fmt.Sprintf("user%d", i),
			password: "wrong",
			remote:   remote,
			xff:      fmt.Sprintf("198.51.100.%d", i+1),
		})
	}

	rec := attemptLogin(handler, loginAttempt{username: "next", password: "wrong", remote: remote, xff: "198.51.100.200"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d: rotating X-Forwarded-For escaped the limit", rec.Code, http.StatusTooManyRequests)
	}
}

func TestLoginLimitsClientsBehindTrustedProxySeparately(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.rc.Security.TrustedProxies = []string{"10.0.0.1"}
	const proxy = "10.0.0.1:1234"

	for i := 0; i < maxLoginAttemptsPerIP; i++ {
		attemptLogin(handler, loginAttempt{
			username: fmt.Sprintf("user%d", i),
			password: "wrong",
			remote:   proxy,
			xff:      "198.51.100.1",
		})
	}

	blocked := attemptLogin(handler, loginAttempt{username: "next", password: "wrong", remote: proxy, xff: "198.51.100.1"})
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("attacker: status = %d, want %d", blocked.Code, http.StatusTooManyRequests)
	}
	other := attemptLogin(handler, loginAttempt{username: "admin", password: "secret", remote: proxy, xff: "198.51.100.2"})
	if findOptionalCookie(other.Result().Cookies(), "sid") == nil {
		t.Fatalf("other client behind the proxy was blocked: status = %d", other.Code)
	}
}

func TestLoginGroupsIPv6ClientsBySubnet(t *testing.T) {
	handler, _ := newTestHandler(t)

	for i := 0; i < maxLoginAttemptsPerIP; i++ {
		attemptLogin(handler, loginAttempt{
			username: fmt.Sprintf("user%d", i),
			password: "wrong",
			remote:   fmt.Sprintf("[2001:db8:1:2::%x]:1234", i+1),
		})
	}

	rec := attemptLogin(handler, loginAttempt{username: "next", password: "wrong", remote: "[2001:db8:1:2::ffff]:1234"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
}
