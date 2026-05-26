package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/velox0/gatekeeper/internal/auth"
	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/middleware"
	"github.com/velox0/gatekeeper/internal/proxy"
	"github.com/velox0/gatekeeper/internal/session"
)

func main() {
	cfgPath := flag.String("config", "config.example.yml", "path to config yaml")
	flag.Parse()

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

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

	fmt.Printf("starting gatekeeper on %s\n", listen)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
