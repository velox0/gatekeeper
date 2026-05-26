// Package server provides the multi-listener Gateway that dispatches
// requests to virtual hosts based on the Host header.
package server

import (
	"context"
	"errors"
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
	"github.com/velox0/gatekeeper/internal/proxy"
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
	Addr        string
	Hosts       map[string]*VirtualHost // server_name → vhost
	DefaultHost *VirtualHost            // fallback server block
	Server      *http.Server
}

// ServeHTTP dispatches incoming requests to the matching virtual host
// based on the Host header. If no exact match is found, it falls back
// to the DefaultHost (if configured), otherwise returning a 404.
func (l *Listener) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	// Strip port from Host header
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(host)

	vhost, ok := l.Hosts[host]
	if !ok {
		if l.DefaultHost != nil {
			vhost = l.DefaultHost
		} else {
			http.Error(w, "no server configured for this host", http.StatusNotFound)
			return
		}
	}
	vhost.Mux.ServeHTTP(w, r)
}

// Gateway orchestrates multiple listeners, each with one or more virtual hosts.
type Gateway struct {
	listeners    []*Listener
	cfg          *config.Config
	mu           sync.Mutex
	reaperCancel context.CancelFunc
}

// NewGateway constructs a Gateway from the loaded config.
// It creates a Listener for each listen address and a VirtualHost
// for each server block, wiring up proxy, auth, and middleware.
func NewGateway(cfg *config.Config) (*Gateway, error) {
	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	gw := &Gateway{cfg: cfg, reaperCancel: reaperCancel}

	if err := plugins.PopulateDefaults(); err != nil {
		log.Printf("warning: failed to seed plugin assets: %v", err)
	}

	const sessionReapInterval = 5 * time.Minute

	for _, lnCfg := range cfg.Listeners {
		ln, err := buildListener(cfg, lnCfg)
		if err != nil {
			reaperCancel()
			return nil, fmt.Errorf("listener %s: %w", lnCfg.Listen, err)
		}
		// Start session reapers for each virtual host.
		for _, vhost := range ln.Hosts {
			vhost.SessStore.StartReaper(reaperCtx, sessionReapInterval)
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

		displayName := rc.ServerName
		if displayName == "" {
			displayName = "<default>"
		}

		upstreamURL, err := url.Parse(rc.Upstream.Target)
		if err != nil {
			return nil, fmt.Errorf("server %s: invalid upstream: %w", displayName, err)
		}

		rev := proxy.NewReverseProxy(upstreamURL)
		sessStore := session.NewInMemoryStore()

		mux := http.NewServeMux()

		authHandler, err := auth.NewHandler(&rc, sessStore)
		if err != nil {
			return nil, fmt.Errorf("server %s: auth init: %w", displayName, err)
		}

		mux.HandleFunc("/login", authHandler.LoginHandler)
		mux.HandleFunc("/logout", authHandler.LogoutHandler)
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.Header().Set("Allow", http.MethodGet)
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
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
		if name != "" {
			ln.Hosts[name] = vhost
		} else {
			ln.Hosts[""] = vhost
		}

		// If explicitly empty/omitted server_name, it becomes the default fallback
		if name == "" {
			ln.DefaultHost = vhost
		}
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

// Start binds all listener sockets synchronously (so bind errors are caught
// immediately) and then serves in background goroutines.
func (gw *Gateway) Start() error {
	// Phase 1: bind all sockets synchronously.
	type boundListener struct {
		listener *Listener
		netLn    net.Listener
	}
	var bound []boundListener

	for _, ln := range gw.listeners {
		netLn, err := net.Listen("tcp", ln.Addr)
		if err != nil {
			// Close any already-bound listeners before returning.
			for _, bl := range bound {
				_ = bl.netLn.Close()
			}
			return fmt.Errorf("listener %s: %w", ln.Addr, err)
		}
		bound = append(bound, boundListener{listener: ln, netLn: netLn})
	}

	// Phase 2: serve on each bound socket in a goroutine.
	for _, bl := range bound {
		go func(l *Listener, nl net.Listener) {
			var serverNames []string
			for name := range l.Hosts {
				if name == "" {
					serverNames = append(serverNames, "<default>")
				} else {
					serverNames = append(serverNames, name)
				}
			}
			log.Printf("listening on %s (vhosts: %s)", nl.Addr().String(), strings.Join(serverNames, ", "))
			if err := l.Server.Serve(nl); err != nil && err != http.ErrServerClosed {
				log.Printf("listener %s error: %v", nl.Addr().String(), err)
			}
		}(bl.listener, bl.netLn)
	}

	return nil
}

// Shutdown gracefully shuts down all listeners concurrently.
func (gw *Gateway) Shutdown(ctx context.Context) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)

	for _, ln := range gw.listeners {
		wg.Add(1)
		go func(s *http.Server) {
			defer wg.Done()
			if err := s.Shutdown(ctx); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(ln.Server)
	}

	wg.Wait()
	gw.reaperCancel()
	return errors.Join(errs...)
}

// Listeners returns the gateway's listeners (for signal handling / status).
func (gw *Gateway) Listeners() []*Listener {
	return gw.listeners
}
