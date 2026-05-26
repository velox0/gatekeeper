package config

// ResolvedConfig holds the fully-merged configuration for a single virtual host.
// It is computed by combining the global Config with a specific ServerBlock.
type ResolvedConfig struct {
	ServerName string
	Listen     string
	Upstream   UpstreamConfig
	Routes     []RouteConfig
	Auth       AuthConfig
	Security   SecurityConfig
	Users      []UserConfig
	Plugins    map[string]bool
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

	rc := ResolvedConfig{
		ServerName: srv.ServerName,
		Listen:     ln.Listen,
		Upstream:   srv.Upstream,
		Routes:     srv.Routes,
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
	rc.Security = c.Security
	if srv.Security != nil {
		rc.Security = *srv.Security
	}

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

// GetResolvedUsers returns a copy of the resolved user list.
func (rc *ResolvedConfig) GetResolvedUsers() []UserConfig {
	users := make([]UserConfig, len(rc.Users))
	copy(users, rc.Users)
	return users
}

// GetResolvedPlugins returns a copy of the resolved plugins map.
func (rc *ResolvedConfig) GetResolvedPlugins() map[string]bool {
	out := make(map[string]bool, len(rc.Plugins))
	for k, v := range rc.Plugins {
		out[k] = v
	}
	return out
}
