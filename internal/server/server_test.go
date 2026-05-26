package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/velox0/gatekeeper/internal/config"
)

func TestListenerRoutesToCorrectVirtualHost(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "app.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:3000"},
					},
					{
						ServerName: "docs.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:4000"},
					},
				},
			},
		},
	}

	gw, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("NewGateway error: %v", err)
	}

	ln := gw.Listeners()[0]

	// Request to app.local should route to its vhost
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Host = "app.local:8080"
	rec := httptest.NewRecorder()
	ln.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("app.local /login: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Request to docs.local should route to its vhost
	req2 := httptest.NewRequest(http.MethodGet, "/login", nil)
	req2.Host = "docs.local:8080"
	rec2 := httptest.NewRecorder()
	ln.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("docs.local /login: status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestListenerReturns404ForUnknownHost(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "app.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:3000"},
					},
				},
			},
		},
	}

	gw, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("NewGateway error: %v", err)
	}

	ln := gw.Listeners()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "unknown.local:8080"
	rec := httptest.NewRecorder()
	ln.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown host: status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestListenerHostMatchingIsCaseInsensitive(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "App.Local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:3000"},
					},
				},
			},
		},
	}

	gw, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("NewGateway error: %v", err)
	}

	ln := gw.Listeners()[0]

	// Send with different case
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Host = "APP.LOCAL"
	rec := httptest.NewRecorder()
	ln.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("case-insensitive match: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestMultipleListenersCreated(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "a.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:3000"},
					},
				},
			},
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "b.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:4000"},
					},
				},
			},
		},
	}

	gw, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("NewGateway error: %v", err)
	}

	if len(gw.Listeners()) != 2 {
		t.Fatalf("listeners = %d, want 2", len(gw.Listeners()))
	}

	// Each listener should have its own vhost
	if _, ok := gw.Listeners()[0].Hosts["a.local"]; !ok {
		t.Error("listener[0] should have vhost a.local")
	}
	if _, ok := gw.Listeners()[1].Hosts["b.local"]; !ok {
		t.Error("listener[1] should have vhost b.local")
	}
}
