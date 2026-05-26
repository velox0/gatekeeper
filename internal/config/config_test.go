package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadConfigAppliesDefaults(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
upstream:
  target: "http://localhost:3000"
`))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Auth.CookieName != "gatekeeper_session" {
		t.Fatalf("CookieName = %q, want gatekeeper_session", cfg.Auth.CookieName)
	}
	if cfg.Auth.SessionTTL != 24*time.Hour {
		t.Fatalf("SessionTTL = %v, want 24h", cfg.Auth.SessionTTL)
	}
}

func TestLoadConfigParsesDuration(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
auth:
  session_ttl: 30m
upstream:
  target: "https://example.com"
`))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Auth.SessionTTL != 30*time.Minute {
		t.Fatalf("SessionTTL = %v, want 30m", cfg.Auth.SessionTTL)
	}
}

func TestLoadConfigValidatesUpstreamTarget(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "missing",
			body: `{}`,
			want: "upstream target is required",
		},
		{
			name: "relative",
			body: `upstream:
  target: "localhost:3000"
`,
			want: "absolute URL",
		},
		{
			name: "unsupported scheme",
			body: `upstream:
  target: "ftp://example.com"
`,
			want: "http or https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, tt.body))
			if err == nil {
				t.Fatal("LoadConfig returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tt.want)
			}
		})
	}
}
