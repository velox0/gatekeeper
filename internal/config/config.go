package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const defaultAppName = "Gatekeeper"

// AuthConfig controls session cookie settings.
type AuthConfig struct {
	CookieName string        `yaml:"cookie_name"`
	SessionTTL time.Duration `yaml:"session_ttl"`
}

// UpstreamConfig holds the backend target URL.
type UpstreamConfig struct {
	Target string `yaml:"target"`
}

// UserConfig holds a single user's credentials.
type UserConfig struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
}

// SecurityConfig controls cookie security flags.
type SecurityConfig struct {
	SecureCookies bool `yaml:"secure_cookies"`
}

// RouteConfig defines a path and its proxy/auth behaviour.
type RouteConfig struct {
	Path     string `yaml:"path"`
	Upstream string `yaml:"upstream"`
	Auth     bool   `yaml:"auth"`
}

// ServerBlock defines a virtual host within a listener.
type ServerBlock struct {
	ServerName string          `yaml:"server_name"`
	Upstream   UpstreamConfig  `yaml:"upstream"`
	Routes     []RouteConfig   `yaml:"routes"`
	Auth       *AuthConfig     `yaml:"auth,omitempty"`
	Security   *SecurityConfig `yaml:"security,omitempty"`
	Users      []UserConfig    `yaml:"users,omitempty"`
	Plugins    map[string]bool `yaml:"plugins,omitempty"`
}

// ListenerConfig groups server blocks that share a listen address.
type ListenerConfig struct {
	Listen  string        `yaml:"listen"`
	Servers []ServerBlock `yaml:"servers"`
}

// Config is the top-level configuration.
// Global-scope values are inherited by every server block unless overridden.
type Config struct {
	mu        sync.RWMutex     `yaml:"-"`
	AppName   string           `yaml:"app_name,omitempty"`
	Auth      AuthConfig       `yaml:"auth"`
	Security  SecurityConfig   `yaml:"security"`
	Users     []UserConfig     `yaml:"users,omitempty"`
	Plugins   map[string]bool  `yaml:"plugins,omitempty"`
	Listeners []ListenerConfig `yaml:"listeners"`
}

// DisplayName returns AppName if set, or the default "Gatekeeper".
func (c *Config) DisplayName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.AppName != "" {
		return c.AppName
	}
	return defaultAppName
}

// GetUsers returns a copy of the global user list under a read lock.
func (c *Config) GetUsers() []UserConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	users := make([]UserConfig, len(c.Users))
	copy(users, c.Users)
	return users
}

// GetPlugins returns a copy of the global plugins map under a read lock.
func (c *Config) GetPlugins() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]bool, len(c.Plugins))
	for k, v := range c.Plugins {
		out[k] = v
	}
	return out
}

// Reload re-reads the config file from disk and swaps all mutable state
// (users, plugins) at both global and server-block level.
func (c *Config) Reload(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reload: failed to read config: %w", err)
	}
	var fresh Config
	if err := yaml.Unmarshal(b, &fresh); err != nil {
		return fmt.Errorf("reload: failed to parse config: %w", err)
	}
	c.mu.Lock()
	c.AppName = fresh.AppName
	c.Users = fresh.Users
	c.Plugins = fresh.Plugins
	c.Listeners = fresh.Listeners
	c.mu.Unlock()
	log.Printf("config reloaded: %d user(s), %d plugin(s), %d listener(s)",
		len(fresh.Users), len(fresh.Plugins), len(fresh.Listeners))
	return nil
}

// ReloadUsers is kept for backward compatibility; it delegates to Reload.
func (c *Config) ReloadUsers(path string) error {
	return c.Reload(path)
}

// SaveConfig marshals the config and writes it to the given path,
// preserving any comments that exist in the original file.
func SaveConfig(cfg *Config, path string) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	// Marshal the current struct into a fresh yaml.Node.
	newBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var newDoc yaml.Node
	if err := yaml.Unmarshal(newBytes, &newDoc); err != nil {
		return fmt.Errorf("failed to re-parse marshaled config: %w", err)
	}

	// Try to read the original file and transfer comments.
	if origBytes, readErr := os.ReadFile(path); readErr == nil {
		var oldDoc yaml.Node
		if yaml.Unmarshal(origBytes, &oldDoc) == nil {
			transferComments(&oldDoc, &newDoc)
		}
	}

	out, err := yaml.Marshal(&newDoc)
	if err != nil {
		return fmt.Errorf("failed to marshal final config: %w", err)
	}
	return os.WriteFile(path, out, 0644)
}

// transferComments recursively copies HeadComment, LineComment, and
// FootComment from an old yaml.Node tree to a new one, matching by
// key name for mappings and by position for sequences.
func transferComments(old, new *yaml.Node) {
	if old == nil || new == nil {
		return
	}

	// Copy comments from old node to new node.
	if old.HeadComment != "" {
		new.HeadComment = old.HeadComment
	}
	if old.LineComment != "" {
		new.LineComment = old.LineComment
	}
	if old.FootComment != "" {
		new.FootComment = old.FootComment
	}

	switch {
	case old.Kind == yaml.DocumentNode && new.Kind == yaml.DocumentNode:
		// Document nodes wrap a single content node.
		for i := 0; i < len(old.Content) && i < len(new.Content); i++ {
			transferComments(old.Content[i], new.Content[i])
		}

	case old.Kind == yaml.MappingNode && new.Kind == yaml.MappingNode:
		// Build index: key name → index of key node in old.Content.
		oldKeys := make(map[string]int)
		for i := 0; i < len(old.Content)-1; i += 2 {
			oldKeys[old.Content[i].Value] = i
		}
		for i := 0; i < len(new.Content)-1; i += 2 {
			key := new.Content[i].Value
			if oi, ok := oldKeys[key]; ok {
				transferComments(old.Content[oi], new.Content[i])     // key node
				transferComments(old.Content[oi+1], new.Content[i+1]) // value node
			}
		}

	case old.Kind == yaml.SequenceNode && new.Kind == yaml.SequenceNode:
		// Transfer comments positionally.
		for i := 0; i < len(old.Content) && i < len(new.Content); i++ {
			transferComments(old.Content[i], new.Content[i])
		}
	}
}

// LoadConfig reads and validates the configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	// defaults
	if cfg.Auth.CookieName == "" {
		cfg.Auth.CookieName = "gatekeeper_session"
	}
	if cfg.Auth.SessionTTL == 0 {
		cfg.Auth.SessionTTL = 24 * time.Hour
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if len(cfg.Listeners) == 0 {
		return fmt.Errorf("at least one listener is required")
	}

	listenAddrs := make(map[string]bool)
	for li, ln := range cfg.Listeners {
		if ln.Listen == "" {
			return fmt.Errorf("listener[%d]: listen address is required", li)
		}
		if listenAddrs[ln.Listen] {
			return fmt.Errorf("duplicate listen address: %s", ln.Listen)
		}
		listenAddrs[ln.Listen] = true

		if len(ln.Servers) == 0 {
			return fmt.Errorf("listener[%d] (%s): at least one server block is required", li, ln.Listen)
		}

		serverNames := make(map[string]bool)
		for si, srv := range ln.Servers {
			if srv.ServerName == "" {
				return fmt.Errorf("listener[%d] (%s) server[%d]: server_name is required", li, ln.Listen, si)
			}
			if serverNames[srv.ServerName] {
				return fmt.Errorf("listener[%d] (%s): duplicate server_name %q", li, ln.Listen, srv.ServerName)
			}
			serverNames[srv.ServerName] = true

			if err := validateUpstream(srv.Upstream, fmt.Sprintf("listener[%d] (%s) server[%d] (%s)", li, ln.Listen, si, srv.ServerName)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateUpstream(up UpstreamConfig, ctx string) error {
	if up.Target == "" {
		return fmt.Errorf("%s: upstream target is required", ctx)
	}
	u, err := url.Parse(up.Target)
	if err != nil {
		return fmt.Errorf("%s: invalid upstream target: %w", ctx, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s: upstream target must be an absolute URL", ctx)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s: upstream target must use http or https", ctx)
	}
	return nil
}
