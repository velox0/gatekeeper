package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/velox0/gatekeeper/internal/config"
	"github.com/velox0/gatekeeper/internal/daemon"
	"github.com/velox0/gatekeeper/internal/server"
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

	// Write PID file
	if err := daemon.WritePID(*pidPath); err != nil {
		log.Fatalf("failed to write PID file: %v", err)
	}
	defer daemon.RemovePID(*pidPath)

	gw, err := server.NewGateway(cfg)
	if err != nil {
		log.Fatalf("failed to build gateway: %v", err)
	}

	// --- Signal handling ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				log.Println("received SIGHUP, reloading configuration...")
				if err := cfg.Reload(*cfgPath); err != nil {
					log.Printf("config reload failed: %v", err)
				}
			case syscall.SIGINT, syscall.SIGTERM:
				log.Printf("received %s, shutting down gracefully...", sig)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := gw.Shutdown(ctx); err != nil {
					log.Printf("graceful shutdown error: %v", err)
				}
				return
			}
		}
	}()

	fmt.Printf("starting %s (pid %d)\n", cfg.DisplayName(), os.Getpid())
	if err := gw.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}

	// Block until a signal stops us.
	// gw.Start() launches goroutines; we need to wait for shutdown.
	select {}
}

// requireRoot exits with an error if the process is not running as root.
func requireRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "error: this command requires root privileges. Please run with sudo.")
		os.Exit(1)
	}
}
