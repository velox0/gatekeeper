package config

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

// ResolvedConfig holds the fully-merged configuration for a single virtual host.
// It is computed by combining the global Config with a specific ServerBlock.
type ResolvedConfig struct {
	mu         *sync.RWMutex
	AppName    string
	ServerName string
	Listen     string
	Upstream   UpstreamConfig
	Auth       AuthConfig
	Security   SecurityConfig
	Users      []UserConfig
	Plugins    map[string]bool
}

func (rc *ResolvedConfig) rlock() {
	if rc.mu != nil {
		rc.mu.RLock()
	}
}

func (rc *ResolvedConfig) runlock() {
	if rc.mu != nil {
		rc.mu.RUnlock()
	}
}

func (rc *ResolvedConfig) lock() {
	if rc.mu != nil {
		rc.mu.Lock()
	}
}

func (rc *ResolvedConfig) unlock() {
	if rc.mu != nil {
		rc.mu.Unlock()
	}
}

// GetAppName returns the app name thread-safely.
func (rc *ResolvedConfig) GetAppName() string {
	rc.rlock()
	defer rc.runlock()
	return rc.AppName
}

// GetCookieName returns the cookie name thread-safely.
func (rc *ResolvedConfig) GetCookieName() string {
	rc.rlock()
	defer rc.runlock()
	return rc.Auth.CookieName
}

// GetSessionTTL returns the session TTL thread-safely.
func (rc *ResolvedConfig) GetSessionTTL() time.Duration {
	rc.rlock()
	defer rc.runlock()
	return rc.Auth.SessionTTL
}

// GetSecureCookies returns whether cookies require secure flag thread-safely.
func (rc *ResolvedConfig) GetSecureCookies() bool {
	rc.rlock()
	defer rc.runlock()
	return secureCookiesOrDefault(rc.Security.SecureCookies)
}

// GetSameSite returns the effective SameSite policy for the session cookie.
func (rc *ResolvedConfig) GetSameSite() http.SameSite {
	rc.rlock()
	defer rc.runlock()
	value := strings.ToLower(strings.TrimSpace(rc.Security.SameSite))
	if value == "" {
		value = defaultSameSite
	}
	switch value {
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteStrictMode
	}
}

// GetAuthorizeFavicon returns whether GET /favicon.ico can bypass auth.
func (rc *ResolvedConfig) GetAuthorizeFavicon() bool {
	rc.rlock()
	defer rc.runlock()
	return authorizeFaviconOrDefault(rc.Security.AuthorizeFavicon)
}

// Update updates all fields of the ResolvedConfig thread-safely.
func (rc *ResolvedConfig) Update(fresh ResolvedConfig) {
	rc.lock()
	defer rc.unlock()
	rc.AppName = fresh.AppName
	rc.ServerName = fresh.ServerName
	rc.Listen = fresh.Listen
	rc.Upstream = fresh.Upstream
	rc.Auth = fresh.Auth
	rc.Security = fresh.Security
	rc.Users = fresh.Users
	rc.Plugins = fresh.Plugins
}

// ResolveServer merges global-scope settings with a server block to produce
// a complete, self-contained config for that virtual host.
//
// Merge rules:
//   - Users:    union (global + local, local can shadow global by username)
//   - Plugins:  merge (global defaults, local overrides individual toggles)
//   - Auth:     override (server-level replaces individual fields if non-zero)
//   - Security: override (server-level replaces if set)
func (c *Config) ResolveServer(ln ListenerConfig, srv ServerBlock) ResolvedConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	appName := c.AppName
	if appName == "" {
		appName = defaultAppName
	}

	rc := ResolvedConfig{
		mu:         new(sync.RWMutex),
		AppName:    appName,
		ServerName: srv.ServerName,
		Listen:     ln.Listen,
		Upstream:   srv.Upstream,
	}

	// --- Auth: start with global, override with server-level ---
	rc.Auth = c.Auth
	if srv.Auth != nil {
		if srv.Auth.CookieName != "" {
			rc.Auth.CookieName = srv.Auth.CookieName
		}
		if srv.Auth.SessionTTL != 0 {
			rc.Auth.SessionTTL = srv.Auth.SessionTTL
		}
	}

	// --- Security: start with global, override with server-level ---
	rc.Security = mergeSecurity(c.Security, srv.Security)

	// --- Users: union (global + local) ---
	// Local users can shadow a global user with the same username.
	seen := make(map[string]bool)
	for _, u := range srv.Users {
		rc.Users = append(rc.Users, u)
		seen[u.Username] = true
	}
	for _, u := range c.Users {
		if !seen[u.Username] {
			rc.Users = append(rc.Users, u)
		}
	}

	// --- Plugins: merge (global base, local overrides) ---
	rc.Plugins = make(map[string]bool)
	for k, v := range c.Plugins {
		rc.Plugins[k] = v
	}
	for k, v := range srv.Plugins {
		rc.Plugins[k] = v
	}

	return rc
}

// GetResolvedUsers returns a copy of the resolved user list thread-safely.
func (rc *ResolvedConfig) GetResolvedUsers() []UserConfig {
	rc.rlock()
	defer rc.runlock()
	users := make([]UserConfig, len(rc.Users))
	copy(users, rc.Users)
	return users
}

// GetResolvedPlugins returns a copy of the resolved plugins map thread-safely.
func (rc *ResolvedConfig) GetResolvedPlugins() map[string]bool {
	rc.rlock()
	defer rc.runlock()
	out := make(map[string]bool, len(rc.Plugins))
	for k, v := range rc.Plugins {
		out[k] = v
	}
	return out
}
