package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/velox0/gatekeeper/internal/accesslog"
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

func TestListenerRoutesToDefaultHost(t *testing.T) {
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
						ServerName: "", // catch-all / default host
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

	// Request with exact match should route to app.local
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	req.Host = "app.local"
	rec := httptest.NewRecorder()
	ln.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("app.local: status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Request with unmatched Host should route to the default catch-all
	req2 := httptest.NewRequest(http.MethodGet, "/login", nil)
	req2.Host = "unmatched.local"
	rec2 := httptest.NewRecorder()
	ln.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("unmatched.local: status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestGatewayReload(t *testing.T) {
	cfg := &config.Config{
		AppName: "OldApp",
		Auth:    config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
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

	vhost := gw.Listeners()[0].Hosts["app.local"]
	if vhost.Config.GetAppName() != "OldApp" {
		t.Errorf("AppName = %q, want OldApp", vhost.Config.GetAppName())
	}
	if vhost.Config.GetSessionTTL() != time.Hour {
		t.Errorf("SessionTTL = %v, want 1h", vhost.Config.GetSessionTTL())
	}

	// Update the global configuration in place and call Reload
	cfg.AppName = "NewApp"
	cfg.Auth.SessionTTL = 2 * time.Hour
	gw.Reload()

	if vhost.Config.GetAppName() != "NewApp" {
		t.Errorf("AppName after reload = %q, want NewApp", vhost.Config.GetAppName())
	}
	if vhost.Config.GetSessionTTL() != 2*time.Hour {
		t.Errorf("SessionTTL after reload = %v, want 2h", vhost.Config.GetSessionTTL())
	}
}

func TestGatewayReloadRevokesRemovedUserSessions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	cfg := &config.Config{
		Auth:  config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Users: []config.UserConfig{{Username: "admin", PasswordHash: string(hash)}},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "app.local",
						Upstream:   config.UpstreamConfig{Target: upstream.URL},
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

	login := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("username=admin&password=secret"))
	login.Host = "app.local"
	login.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	ln.ServeHTTP(loginRec, login)
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login cookies = %d, want 1 (status %d)", len(cookies), loginRec.Code)
	}

	get := func() int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = "app.local"
		req.AddCookie(cookies[0])
		rec := httptest.NewRecorder()
		ln.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := get(); code != http.StatusNoContent {
		t.Fatalf("status before removal = %d, want %d", code, http.StatusNoContent)
	}

	cfg.Users = nil
	gw.Reload()

	if code := get(); code != http.StatusSeeOther {
		t.Fatalf("status after removal = %d, want %d", code, http.StatusSeeOther)
	}
}

func TestAccessLogRecordsResolvedClientIP(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "access.log")
	t.Setenv(accesslog.EnvPath, logPath)

	cfg := &config.Config{
		Auth:     config.AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Security: config.SecurityConfig{TrustedProxies: []string{"10.0.0.1"}},
		Listeners: []config.ListenerConfig{
			{
				Listen: ":0",
				Servers: []config.ServerBlock{
					{
						ServerName: "app.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:3000"},
					},
					{
						ServerName: "direct.local",
						Upstream:   config.UpstreamConfig{Target: "http://localhost:4000"},
						Security:   &config.SecurityConfig{TrustedProxies: []string{}},
					},
				},
			},
		},
	}

	gw, err := NewGateway(cfg)
	if err != nil {
		t.Fatalf("NewGateway error: %v", err)
	}
	t.Cleanup(func() {
		if err := gw.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown error: %v", err)
		}
	})
	handler := gw.Listeners()[0].Server.Handler

	cases := []struct {
		name       string
		host       string
		remote     string
		wantStatus int
		want       string
	}{
		{name: "trusted proxy", host: "app.local", remote: "10.0.0.1:5555", wantStatus: http.StatusOK, want: "198.51.100.7"},
		{name: "untrusted peer", host: "app.local", remote: "203.0.113.9:5555", wantStatus: http.StatusOK, want: "203.0.113.9"},
		{name: "vhost without trusted proxies", host: "direct.local", remote: "10.0.0.1:5555", wantStatus: http.StatusOK, want: "10.0.0.1"},
		{name: "unmatched host", host: "unknown.local", remote: "10.0.0.1:5555", wantStatus: http.StatusNotFound, want: "10.0.0.1"},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Host = tc.host
		req.RemoteAddr = tc.remote
		req.Header.Set("X-Forwarded-For", "198.51.100.7")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.wantStatus {
			t.Fatalf("%s: status = %d, want %d", tc.name, rec.Code, tc.wantStatus)
		}
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open access log: %v", err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for _, tc := range cases {
		var got struct {
			RemoteAddr string `json:"remote_addr"`
			Host       string `json:"host"`
		}
		if err := dec.Decode(&got); err != nil {
			t.Fatalf("%s: decode access log entry: %v", tc.name, err)
		}
		if got.Host != tc.host || got.RemoteAddr != tc.want {
			t.Errorf("%s: logged host %q remote_addr %q, want host %q remote_addr %q", tc.name, got.Host, got.RemoteAddr, tc.host, tc.want)
		}
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected extra access log entry: %s (err %v)", extra, err)
	}
}
