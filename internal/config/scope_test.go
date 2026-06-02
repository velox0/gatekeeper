package config

import (
	"testing"
	"time"
)

func TestResolveServerUsersUnion(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Users: []UserConfig{
			{Username: "global_admin", PasswordHash: "hash1"},
			{Username: "shared", PasswordHash: "global_hash"},
		},
	}

	ln := ListenerConfig{Listen: ":8080"}
	srv := ServerBlock{
		ServerName: "app.local",
		Upstream:   UpstreamConfig{Target: "http://localhost:3000"},
		Users: []UserConfig{
			{Username: "local_dev", PasswordHash: "hash2"},
			{Username: "shared", PasswordHash: "local_hash"}, // shadows global
		},
	}

	rc := cfg.ResolveServer(ln, srv)

	if len(rc.Users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(rc.Users))
	}

	// "shared" should come from local (shadows global)
	byName := make(map[string]string)
	for _, u := range rc.Users {
		byName[u.Username] = u.PasswordHash
	}
	if byName["shared"] != "local_hash" {
		t.Errorf("shared user should use local hash, got %q", byName["shared"])
	}
	if _, ok := byName["global_admin"]; !ok {
		t.Error("global_admin should be present")
	}
	if _, ok := byName["local_dev"]; !ok {
		t.Error("local_dev should be present")
	}
}

func TestResolveServerPluginsMerge(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Plugins: map[string]bool{
			"hearts": true,
			"matrix": false,
			"aurora": true,
		},
	}

	ln := ListenerConfig{Listen: ":8080"}
	srv := ServerBlock{
		ServerName: "app.local",
		Upstream:   UpstreamConfig{Target: "http://localhost:3000"},
		Plugins: map[string]bool{
			"matrix": true,  // override: enable
			"aurora": false, // override: disable
		},
	}

	rc := cfg.ResolveServer(ln, srv)

	want := map[string]bool{"hearts": true, "matrix": true, "aurora": false}
	for k, wantV := range want {
		if rc.Plugins[k] != wantV {
			t.Errorf("plugin %q = %v, want %v", k, rc.Plugins[k], wantV)
		}
	}
}

func TestResolveServerAuthOverride(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{CookieName: "gk_global", SessionTTL: 24 * time.Hour},
	}

	ln := ListenerConfig{Listen: ":8080"}
	srv := ServerBlock{
		ServerName: "app.local",
		Upstream:   UpstreamConfig{Target: "http://localhost:3000"},
		Auth:       &AuthConfig{CookieName: "gk_local"},
		// SessionTTL left as zero → should inherit global
	}

	rc := cfg.ResolveServer(ln, srv)

	if rc.Auth.CookieName != "gk_local" {
		t.Errorf("CookieName = %q, want gk_local", rc.Auth.CookieName)
	}
	if rc.Auth.SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v, want 24h (inherited)", rc.Auth.SessionTTL)
	}
}

func TestResolveServerSecurityOverride(t *testing.T) {
	cfg := &Config{
		Auth:     AuthConfig{CookieName: "gk", SessionTTL: time.Hour},
		Security: SecurityConfig{SecureCookies: false, AuthorizeFavicon: false},
	}

	ln := ListenerConfig{Listen: ":8080"}
	srv := ServerBlock{
		ServerName: "app.local",
		Upstream:   UpstreamConfig{Target: "http://localhost:3000"},
		Security:   &SecurityConfig{SecureCookies: true, AuthorizeFavicon: true},
	}

	rc := cfg.ResolveServer(ln, srv)

	if !rc.Security.SecureCookies {
		t.Error("SecureCookies should be true (server override)")
	}
	if !rc.Security.AuthorizeFavicon {
		t.Error("AuthorizeFavicon should be true (server override)")
	}
}

func TestResolveServerInheritsGlobalDefaults(t *testing.T) {
	cfg := &Config{
		Auth:     AuthConfig{CookieName: "gk_global", SessionTTL: 12 * time.Hour},
		Security: SecurityConfig{SecureCookies: true, AuthorizeFavicon: false},
		Users:    []UserConfig{{Username: "admin", PasswordHash: "hash"}},
		Plugins:  map[string]bool{"hearts": true},
	}

	ln := ListenerConfig{Listen: ":9090"}
	srv := ServerBlock{
		ServerName: "bare.local",
		Upstream:   UpstreamConfig{Target: "http://localhost:5000"},
		// No overrides at all
	}

	rc := cfg.ResolveServer(ln, srv)

	if rc.Auth.CookieName != "gk_global" {
		t.Errorf("CookieName = %q, want gk_global", rc.Auth.CookieName)
	}
	if !rc.Security.SecureCookies {
		t.Error("SecureCookies should be true (inherited)")
	}
	if rc.Security.AuthorizeFavicon {
		t.Error("AuthorizeFavicon should be false (inherited)")
	}
	if len(rc.Users) != 1 || rc.Users[0].Username != "admin" {
		t.Errorf("Users = %v, want [admin]", rc.Users)
	}
	if !rc.Plugins["hearts"] {
		t.Error("hearts plugin should be enabled (inherited)")
	}
}
