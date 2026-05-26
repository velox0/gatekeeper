package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/velox0/gatekeeper/internal/auth"
	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/daemon"
	"github.com/velox0/gatekeeper/internal/middleware"
	"github.com/velox0/gatekeeper/internal/plugins"
	"github.com/velox0/gatekeeper/internal/proxy"
	"github.com/velox0/gatekeeper/internal/session"
)

func main() {
	cfgPath := flag.String("config", "config.example.yml", "path to config yaml")
	pidPath := flag.String("pid", daemon.DefaultPIDPath, "path to PID file")
	flag.Parse()

	args := flag.Args()

	// --- Management mode ---
	if len(args) > 0 && args[0] == "users" {
		requireRoot()
		config.HandleUsersCommand(*cfgPath, *pidPath, args[1:])
		return
	}

	if len(args) > 0 && args[0] == "plugin" {
		config.HandlePluginsCommand(*cfgPath, *pidPath, args[1:])
		return
	}

	// --- Daemon mode ---
	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Seed default plugin assets to ~/.gatekeeper/ on first run
	if err := plugins.PopulateDefaults(); err != nil {
		log.Printf("warning: failed to seed plugin assets: %v", err)
	}

	// Write PID file
	if err := daemon.WritePID(*pidPath); err != nil {
		log.Fatalf("failed to write PID file: %v", err)
	}
	defer daemon.RemovePID(*pidPath)

	sessStore := session.NewInMemoryStore()

	upstreamURL, err := url.Parse(cfg.Upstream.Target)
	if err != nil {
		log.Fatalf("invalid upstream target: %v", err)
	}

	rev := proxy.NewReverseProxy(upstreamURL)

	mux := middleware.NewMux()

	// auth handlers
	authHandler, err := auth.NewHandler(cfg, sessStore)
	if err != nil {
		log.Fatalf("failed to initialize auth handler: %v", err)
	}
	mux.HandleFunc("/login", authHandler.LoginHandler)
	mux.HandleFunc("/logout", authHandler.LogoutHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Printf("failed to write health response: %v", err)
		}
	})

	// catch-all proxy with auth middleware
	proxyHandler := middleware.RequireAuth(rev, cfg, sessStore)
	mux.Handle("/", proxyHandler)

	listen := cfg.Server.Listen
	if listen == "" {
		listen = ":8080"
	}

	srv := &http.Server{
		Addr:         listen,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Signal handling ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				log.Println("received SIGHUP, reloading configuration...")
				if err := cfg.ReloadUsers(*cfgPath); err != nil {
					log.Printf("config reload failed: %v", err)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("received %s, shutting down gracefully...", sig)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := srv.Shutdown(ctx); err != nil {
					log.Printf("graceful shutdown error: %v", err)
				}
				return
			}
		}
	}()

	fmt.Printf("starting gatekeeper on %s (pid %d)\n", listen, os.Getpid())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
	log.Println("gatekeeper stopped")
}

// requireRoot exits with an error if the process is not running as root.
func requireRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "error: this command requires root privileges. Please run with sudo.")
		os.Exit(1)
	}
}
