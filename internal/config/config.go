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

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type AuthConfig struct {
	CookieName string        `yaml:"cookie_name"`
	SessionTTL time.Duration `yaml:"session_ttl"`
}

type UpstreamConfig struct {
	Target string `yaml:"target"`
}

type UserConfig struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"password_hash"`
}

type SecurityConfig struct {
	SecureCookies bool `yaml:"secure_cookies"`
}

type RouteConfig struct {
	Path     string `yaml:"path"`
	Upstream string `yaml:"upstream"`
	Auth     bool   `yaml:"auth"`
}

type Config struct {
	mu       sync.RWMutex      `yaml:"-"`
	Server   ServerConfig      `yaml:"server"`
	Auth     AuthConfig        `yaml:"auth"`
	Upstream UpstreamConfig    `yaml:"upstream"`
	Users    []UserConfig      `yaml:"users"`
	Security SecurityConfig    `yaml:"security"`
	Routes   []RouteConfig     `yaml:"routes"`
	Plugins  map[string]bool   `yaml:"plugins,omitempty"`
}

// GetUsers returns the user list under a read lock.
func (c *Config) GetUsers() []UserConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	users := make([]UserConfig, len(c.Users))
	copy(users, c.Users)
	return users
}

// GetPlugins returns a copy of the plugins map under a read lock.
func (c *Config) GetPlugins() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]bool, len(c.Plugins))
	for k, v := range c.Plugins {
		out[k] = v
	}
	return out
}

// ReloadUsers re-reads the config file from disk and swaps the user list.
func (c *Config) ReloadUsers(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reload: failed to read config: %w", err)
	}
	var fresh Config
	if err := yaml.Unmarshal(b, &fresh); err != nil {
		return fmt.Errorf("reload: failed to parse config: %w", err)
	}
	c.mu.Lock()
	c.Users = fresh.Users
	c.Plugins = fresh.Plugins
	c.mu.Unlock()
	log.Printf("config reloaded: %d user(s), %d plugin(s) loaded", len(fresh.Users), len(fresh.Plugins))
	return nil
}

// SaveConfig marshals the config and writes it to the given path.
func SaveConfig(cfg *Config, path string) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, b, 0644)
}

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
	if cfg.Upstream.Target == "" {
		return fmt.Errorf("upstream target is required")
	}

	upstreamURL, err := url.Parse(cfg.Upstream.Target)
	if err != nil {
		return fmt.Errorf("invalid upstream target: %w", err)
	}
	if upstreamURL.Scheme == "" || upstreamURL.Host == "" {
		return fmt.Errorf("upstream target must be an absolute URL")
	}
	if upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https" {
		return fmt.Errorf("upstream target must use http or https")
	}

	return nil
}
