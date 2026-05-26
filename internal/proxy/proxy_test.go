package proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDirectorSetsForwardedProtoFromIncomingRequest(t *testing.T) {
	target, err := url.Parse("http://upstream.example")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	rp := NewReverseProxy(target)
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
	rp := NewReverseProxy(target)
	req := httptest.NewRequest(http.MethodGet, "http://app.example/path", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	rp.Director(req)

	if got := req.Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Fatalf("X-Forwarded-Proto = %q, want existing value https", got)
	}
}
