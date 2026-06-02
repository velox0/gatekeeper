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

const Version = "0.1.6"

func main() {
	cfgPath := flag.String("config", "/etc/gatekeeper/config.yml", "path to config yaml")
	pidPath := flag.String("pid", daemon.DefaultPIDPath, "path to PID file")
	flag.Usage = printGeneralHelp
	flag.Parse()

	args := flag.Args()

	// --- Management & Help modes ---
	if len(args) > 0 {
		switch args[0] {
		case "help":
			printGeneralHelp()
			return
		case "user", "users":
			requireRoot()
			config.HandleUserCommand(*cfgPath, *pidPath, args[1:])
			return
		case "plugin", "plugins":
			config.HandlePluginCommand(*cfgPath, *pidPath, args[1:])
			return
		case "config":
			config.HandleConfigCommand(args[1:])
			return
		case "reload":
			if err := daemon.SignalReload(*pidPath); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("sent reload signal to daemon")
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command: %s\nRun 'gatekeeper help' for usage.\n", args[0])
			os.Exit(1)
		}
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
	if err := daemon.WriteConfigPath(*pidPath, *cfgPath); err != nil {
		daemon.RemovePID(*pidPath)
		log.Fatalf("failed to write config path metadata: %v", err)
	}
	defer daemon.RemoveConfigPath(*pidPath)

	gw, err := server.NewGateway(cfg)
	if err != nil {
		log.Fatalf("failed to build gateway: %v", err)
	}

	// --- Signal handling ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("starting %s v%s (pid %d)\n", cfg.DisplayName(), Version, os.Getpid())
	if err := gw.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}

	// Block until a signal stops us.
	for sig := range sigCh {
		switch sig {
		case syscall.SIGHUP:
			log.Println("received SIGHUP, reloading configuration...")
			if err := cfg.Reload(*cfgPath); err != nil {
				log.Printf("config reload failed: %v", err)
			} else {
				gw.Reload()
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
}

// requireRoot exits with an error if the process is not running as root.
func requireRoot() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "error: this command requires root privileges. Please run with sudo.")
		os.Exit(1)
	}
}

// printGeneralHelp prints usage and command options to the terminal.
func printGeneralHelp() {
	fmt.Printf("Gatekeeper v%s - lightweight reverse proxy & session authentication gateway\n", Version)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gatekeeper [flags]                    Start the gateway daemon")
	fmt.Println("  gatekeeper [flags] user <command>     Manage gateway users")
	fmt.Println("  gatekeeper [flags] plugin <command>   Manage visual login plugins")
	fmt.Println("  gatekeeper [flags] config <command>   Manage configuration & service setup")
	fmt.Println("  gatekeeper [flags] reload             Reload running daemon configuration")
	fmt.Println("  gatekeeper help                       Show this help message")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -config string   path to config yaml (default \"/etc/gatekeeper/config.yml\")")
	fmt.Println("  -pid string      path to PID file (default \"/var/run/gatekeeper.pid\")")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  user    Run 'gatekeeper user help' for user management commands")
	fmt.Println("  plugin  Run 'gatekeeper plugin help' for plugin management commands")
	fmt.Println("  config  Run 'gatekeeper config help' for configuration & service setup commands")
	fmt.Println("  reload  Reload the running daemon's configuration")
}
