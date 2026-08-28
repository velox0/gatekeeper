package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDirectorSetsForwardedProtoFromIncomingRequest(t *testing.T) {
	target, err := url.Parse("http://upstream.example")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	rp := NewReverseProxy(target, func() string { return "Gatekeeper" })
	req := httptest.NewRequest(http.MethodGet, "https://app.example/path", nil)

	rp.Director(req)

	if got := req.URL.Scheme; got != "http" {
		t.Fatalf("URL scheme = %q, want upstream scheme http", got)
	}
	if got := req.Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Fatalf("X-Forwarded-Proto = %q, want https", got)
	}
}

func TestDirectorPreservesExistingForwardedProto(t *testing.T) {
	target, err := url.Parse("http://upstream.example")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	rp := NewReverseProxy(target, func() string { return "Gatekeeper" })
	req := httptest.NewRequest(http.MethodGet, "http://app.example/path", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	rp.Director(req)

	if got := req.Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Fatalf("X-Forwarded-Proto = %q, want existing value https", got)
	}
}

func TestProxyErrorRendersBuiltInBadGatewayPage(t *testing.T) {
	target, err := url.Parse("http://upstream.example")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	rp := NewReverseProxy(target, func() string { return "Test Portal" })
	rp.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("upstream unavailable")
	})

	rec := httptest.NewRecorder()
	rp.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.example/dashboard", nil))

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	for _, want := range []string{"Bad gateway", "Test Portal"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Errorf("body does not contain %q", want)
		}
	}
}

func TestProxyTimeoutRendersBuiltInGatewayTimeoutPage(t *testing.T) {
	target, err := url.Parse("http://upstream.example")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	rp := NewReverseProxy(target, func() string { return "Test Portal" })
	rp.Transport = roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})

	rec := httptest.NewRecorder()
	rp.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://app.example/dashboard", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGatewayTimeout)
	}
	if !strings.Contains(rec.Body.String(), "Gateway timeout") {
		t.Fatal("body does not contain gateway timeout title")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
