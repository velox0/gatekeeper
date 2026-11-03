package config

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted := []string{"10.0.0.1", "192.168.0.0/16"}
	cases := []struct {
		name    string
		trusted []string
		remote  string
		xff     []string
		want    string
	}{
		{name: "no proxies configured", remote: "203.0.113.5:1234", want: "203.0.113.5"},
		{name: "forwarded header ignored without trust", remote: "203.0.113.5:1234", xff: []string{"1.1.1.1"}, want: "203.0.113.5"},
		{name: "forwarded header ignored from untrusted peer", trusted: trusted, remote: "203.0.113.5:1234", xff: []string{"1.1.1.1"}, want: "203.0.113.5"},
		{name: "client behind trusted proxy", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"198.51.100.7"}, want: "198.51.100.7"},
		{name: "forged entries are skipped", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"1.1.1.1, 198.51.100.7"}, want: "198.51.100.7"},
		{name: "chain of trusted proxies", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"198.51.100.7, 192.168.4.4"}, want: "198.51.100.7"},
		{name: "repeated headers", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"1.1.1.1", "198.51.100.7"}, want: "198.51.100.7"},
		{name: "missing header falls back to proxy", trusted: trusted, remote: "10.0.0.1:1234", want: "10.0.0.1"},
		{name: "only trusted hops", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"192.168.4.4"}, want: "192.168.4.4"},
		{name: "malformed hop stops the walk", trusted: trusted, remote: "10.0.0.1:1234", xff: []string{"198.51.100.7, garbage, 192.168.4.4"}, want: "192.168.4.4"},
		{name: "ipv4-mapped peer", trusted: trusted, remote: "[::ffff:10.0.0.1]:1234", xff: []string{"198.51.100.7"}, want: "198.51.100.7"},
		{name: "ipv6 client", trusted: []string{"2001:db8::/32"}, remote: "[2001:db8::1]:1234", xff: []string{"2001:db9::5"}, want: "2001:db9::5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc := &ResolvedConfig{Security: SecurityConfig{TrustedProxies: tc.trusted}}
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for _, value := range tc.xff {
				req.Header.Add("X-Forwarded-For", value)
			}
			if got := rc.ClientIP(req).String(); got != tc.want {
				t.Fatalf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidTrustedProxy(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `
security:
  trusted_proxies: ["not-an-ip"]
listeners:
  - listen: ":8080"
    servers:
      - upstream:
          target: "http://localhost:4000"
`))
	if err == nil || !strings.Contains(err.Error(), "trusted_proxies") {
		t.Fatalf("LoadConfig error = %v, want trusted_proxies error", err)
	}
}

func TestLoadConfigRejectsInvalidServerTrustedProxy(t *testing.T) {
	_, err := LoadConfig(writeConfig(t, `
listeners:
  - listen: ":8080"
    servers:
      - upstream:
          target: "http://localhost:4000"
        security:
          trusted_proxies: ["10.0.0.0/99"]
`))
	if err == nil || !strings.Contains(err.Error(), "trusted_proxies") {
		t.Fatalf("LoadConfig error = %v, want trusted_proxies error", err)
	}
}

func TestResolveServerOverridesTrustedProxies(t *testing.T) {
	cfg := &Config{Security: SecurityConfig{TrustedProxies: []string{"10.0.0.1"}}}

	inherited := cfg.ResolveServer(ListenerConfig{}, ServerBlock{})
	if len(inherited.Security.TrustedProxies) != 1 || inherited.Security.TrustedProxies[0] != "10.0.0.1" {
		t.Fatalf("inherited = %v, want global value", inherited.Security.TrustedProxies)
	}

	overridden := cfg.ResolveServer(ListenerConfig{}, ServerBlock{
		Security: &SecurityConfig{TrustedProxies: []string{}},
	})
	if len(overridden.Security.TrustedProxies) != 0 {
		t.Fatalf("overridden = %v, want empty", overridden.Security.TrustedProxies)
	}
}
