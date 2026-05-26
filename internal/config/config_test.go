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
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
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
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
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
			body: `
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: ""
`,
			want: "upstream target is required",
		},
		{
			name: "relative",
			body: `
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "localhost:3000"
`,
			want: "absolute URL",
		},
		{
			name: "unsupported scheme",
			body: `
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
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

func TestLoadConfigRequiresListeners(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `{}`))
	if err == nil {
		t.Fatal("LoadConfig should fail with no listeners")
	}
	if !strings.Contains(err.Error(), "at least one listener") {
		t.Fatalf("error = %q, want 'at least one listener'", err.Error())
	}
}

func TestLoadConfigRejectsDuplicateListenAddr(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - server_name: a.local
        upstream:
          target: "http://localhost:3000"
  - listen: ":8080"
    servers:
      - server_name: b.local
        upstream:
          target: "http://localhost:4000"
`))
	if err == nil {
		t.Fatal("LoadConfig should fail with duplicate listen address")
	}
	if !strings.Contains(err.Error(), "duplicate listen address") {
		t.Fatalf("error = %q, want 'duplicate listen address'", err.Error())
	}
}

func TestLoadConfigRejectsDuplicateServerName(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "http://localhost:3000"
      - server_name: app.local
        upstream:
          target: "http://localhost:4000"
`))
	if err == nil {
		t.Fatal("LoadConfig should fail with duplicate server_name")
	}
	if !strings.Contains(err.Error(), "duplicate server_name") {
		t.Fatalf("error = %q, want 'duplicate server_name'", err.Error())
	}
}

func TestLoadConfigMultipleListenersAndServers(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
auth:
  cookie_name: gk
users:
  - username: admin
    password_hash: hash
plugins:
  hearts: true
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "http://localhost:3000"
      - server_name: docs.local
        upstream:
          target: "http://localhost:4000"
  - listen: ":9090"
    servers:
      - server_name: admin.local
        upstream:
          target: "http://localhost:5000"
        users:
          - username: superadmin
            password_hash: shash
`))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if len(cfg.Listeners) != 2 {
		t.Fatalf("listeners = %d, want 2", len(cfg.Listeners))
	}
	if len(cfg.Listeners[0].Servers) != 2 {
		t.Fatalf("listener[0].servers = %d, want 2", len(cfg.Listeners[0].Servers))
	}
	if cfg.Listeners[1].Servers[0].ServerName != "admin.local" {
		t.Fatalf("listener[1].server[0].server_name = %q, want admin.local", cfg.Listeners[1].Servers[0].ServerName)
	}
}

func TestSaveConfigPreservesComments(t *testing.T) {
	body := `# This is a top-level comment
app_name: MyApp
auth:
  cookie_name: gk # inline comment
  session_ttl: 1h
# Security section
security:
  secure_cookies: false
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "http://localhost:3000"
`
	path := writeConfig(t, body)
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	// Modify and save
	if cfg.Plugins == nil {
		cfg.Plugins = make(map[string]bool)
	}
	cfg.Plugins["hearts"] = true

	if err := SaveConfig(cfg, path); err != nil {
		t.Fatalf("SaveConfig error: %v", err)
	}

	// Read back and check comments survived
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	content := string(saved)

	if !strings.Contains(content, "# This is a top-level comment") {
		t.Error("head comment was lost")
	}
	if !strings.Contains(content, "# inline comment") {
		t.Error("inline comment was lost")
	}
	if !strings.Contains(content, "# Security section") {
		t.Error("section comment was lost")
	}
}

func TestLoadConfigParsesAppName(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
app_name: MyPortal
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "http://localhost:3000"
`))
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.AppName != "MyPortal" {
		t.Fatalf("AppName = %q, want MyPortal", cfg.AppName)
	}
	if cfg.DisplayName() != "MyPortal" {
		t.Fatalf("DisplayName() = %q, want MyPortal", cfg.DisplayName())
	}
}

func TestDisplayNameDefaultsToGatekeeper(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - server_name: app.local
        upstream:
          target: "http://localhost:3000"
`))
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.AppName != "" {
		t.Fatalf("AppName = %q, want empty", cfg.AppName)
	}
	if cfg.DisplayName() != "Gatekeeper" {
		t.Fatalf("DisplayName() = %q, want Gatekeeper", cfg.DisplayName())
	}
}

func TestLoadConfigAllowsOptionalServerName(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - upstream:
          target: "http://localhost:3000"
`))
	if err != nil {
		t.Fatalf("expected LoadConfig to succeed with empty/omitted server_name: %v", err)
	}
	if cfg.Listeners[0].Servers[0].ServerName != "" {
		t.Fatalf("expected server_name to be empty, got %q", cfg.Listeners[0].Servers[0].ServerName)
	}
}

func TestLoadConfigRejectsMultipleOptionalServerNames(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - upstream:
          target: "http://localhost:3000"
      - upstream:
          target: "http://localhost:4000"
`))
	if err == nil {
		t.Fatal("expected LoadConfig to reject multiple server blocks without server_name")
	}
	if !strings.Contains(err.Error(), "multiple default/catch-all server blocks") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

