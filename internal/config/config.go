package config

import (
	"fmt"
	"net/url"
	"os"
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
	Server   ServerConfig   `yaml:"server"`
	Auth     AuthConfig     `yaml:"auth"`
	Upstream UpstreamConfig `yaml:"upstream"`
	Users    []UserConfig   `yaml:"users"`
	Security SecurityConfig `yaml:"security"`
	Routes   []RouteConfig  `yaml:"routes"`
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
