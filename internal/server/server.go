// Package server provides the multi-listener Gateway that dispatches
// requests to virtual hosts based on the Host header.
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/velox0/gatekeeper/internal/auth"
	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/middleware"
	"github.com/velox0/gatekeeper/internal/plugins"
	"github.com/velox0/gatekeeper/internal/session"
)

// VirtualHost represents a single server_name block with its own
// mux, reverse proxy, auth handler, and session store.
type VirtualHost struct {
	Config    config.ResolvedConfig
	Mux       *http.ServeMux
	Proxy     *httputil.ReverseProxy
	Auth      *auth.Handler
	SessStore *session.InMemoryStore
}

// Listener binds a listen address to a set of virtual hosts keyed by server_name.
type Listener struct {
	Addr   string
	Hosts  map[string]*VirtualHost // server_name → vhost
	Server *http.Server
}

// ServeHTTP dispatches incoming requests to the matching virtual host
// based on the Host header. Unknown hosts receive a 404.
func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// Strip port from Host header
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(host)

	vhost, ok := l.Hosts[host]
	if !ok {
		http.Error(w, "no server configured for this host", http.StatusNotFound)
		return
	}
	vhost.Mux.ServeHTTP(w, r)
}

// Gateway orchestrates multiple listeners, each with one or more virtual hosts.
type Gateway struct {
	listeners []*Listener
	cfg       *config.Config
	mu        sync.Mutex
}

// NewGateway constructs a Gateway from the loaded config.
// It creates a Listener for each listen address and a VirtualHost
// for each server block, wiring up proxy, auth, and middleware.
func NewGateway(cfg *config.Config) (*Gateway, error) {
	gw := &Gateway{cfg: cfg}

	if err := plugins.PopulateDefaults(); err != nil {
		log.Printf("warning: failed to seed plugin assets: %v", err)
	}

	for _, lnCfg := range cfg.Listeners {
		ln, err := buildListener(cfg, lnCfg)
		if err != nil {
			return nil, fmt.Errorf("listener %s: %w", lnCfg.Listen, err)
		}
		gw.listeners = append(gw.listeners, ln)
	}

	return gw, nil
}

func buildListener(cfg *config.Config, lnCfg config.ListenerConfig) (*Listener, error) {
	ln := &Listener{
		Addr:  lnCfg.Listen,
		Hosts: make(map[string]*VirtualHost),
	}

	for _, srvCfg := range lnCfg.Servers {
		rc := cfg.ResolveServer(lnCfg, srvCfg)

		upstreamURL, err := url.Parse(rc.Upstream.Target)
		if err != nil {
			return nil, fmt.Errorf("server %s: invalid upstream: %w", rc.ServerName, err)
		}

		rev := newReverseProxy(upstreamURL)
		sessStore := session.NewInMemoryStore()

		mux := http.NewServeMux()

		authHandler, err := auth.NewHandler(&rc, sessStore)
		if err != nil {
			return nil, fmt.Errorf("server %s: auth init: %w", rc.ServerName, err)
		}

		mux.HandleFunc("/login", authHandler.LoginHandler)
		mux.HandleFunc("/logout", authHandler.LogoutHandler)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
				log.Printf("failed to write health response: %v", err)
			}
		})

		proxyHandler := middleware.RequireAuth(rev, &rc, sessStore)
		mux.Handle("/", proxyHandler)

		vhost := &VirtualHost{
			Config:    rc,
			Mux:       mux,
			Proxy:     rev,
			Auth:      authHandler,
			SessStore: sessStore,
		}

		name := strings.ToLower(srvCfg.ServerName)
		ln.Hosts[name] = vhost
	}

	ln.Server = &http.Server{
		Addr:         ln.Addr,
		Handler:      ln,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return ln, nil
}

// newReverseProxy creates a configured reverse proxy for the given target.
// (mirrors the logic from internal/proxy but avoids a circular import)
func newReverseProxy(target *url.URL) *httputil.ReverseProxy {
	rp := httputil.NewSingleHostReverseProxy(target)

	rp.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	originalDirector := rp.Director
	rp.Director = func(r *http.Request) {
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		originalDirector(r)
		if r.Header.Get("X-Forwarded-Proto") == "" {
			r.Header.Set("X-Forwarded-Proto", proto)
		}
	}

	return rp
}

// Start launches all listeners in separate goroutines.
// It blocks until all listeners have started (or one fails to bind).
func (gw *Gateway) Start() error {
	errc := make(chan error, len(gw.listeners))

	for _, ln := range gw.listeners {
		go func(l *Listener) {
			var serverNames []string
			for name := range l.Hosts {
				serverNames = append(serverNames, name)
			}
			log.Printf("listening on %s (vhosts: %s)", l.Addr, strings.Join(serverNames, ", "))
			if err := l.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errc <- fmt.Errorf("listener %s: %w", l.Addr, err)
			}
		}(ln)
	}

	// Give listeners a moment to fail on bind errors.
	// In production you'd want a readiness signal; this is pragmatic.
	select {
	case err := <-errc:
		return err
	default:
		return nil
	}
}

// Shutdown gracefully shuts down all listeners.
func (gw *Gateway) Shutdown(ctx context.Context) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	var firstErr error
	for _, ln := range gw.listeners {
		if err := ln.Server.Shutdown(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Listeners returns the gateway's listeners (for signal handling / status).
func (gw *Gateway) Listeners() []*Listener {
	return gw.listeners
}
