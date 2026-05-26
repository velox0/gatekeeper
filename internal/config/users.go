package config

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/velox0/gatekeeper/internal/daemon"
)

// HandleUsersCommand parses and dispatches user management subcommands.
// Usage:
//
//	gatekeeper users add <username> <password>
//	gatekeeper users remove <username>
//	gatekeeper users update <username> <new_password>
func HandleUsersCommand(cfgPath, pidPath string, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gatekeeper users <add|remove|update> ...")
		os.Exit(1)
	}

	subcmd := args[0]
	rest := args[1:]

	var err error
	switch subcmd {
	case "add":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper users add <username> <password>")
			os.Exit(1)
		}
		err = AddUser(cfgPath, rest[0], rest[1])
	case "remove":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper users remove <username>")
			os.Exit(1)
		}
		err = RemoveUser(cfgPath, rest[0])
	case "update":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper users update <username> <new_password>")
			os.Exit(1)
		}
		err = UpdateUser(cfgPath, rest[0], rest[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown users subcommand: %s\n", subcmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Signal the running daemon to reload config
	if sigErr := daemon.SignalReload(pidPath); sigErr != nil {
		log.Printf("warning: config saved but could not signal daemon: %v", sigErr)
		log.Printf("the daemon will pick up changes on next restart")
	} else {
		log.Println("config saved and daemon signaled to reload")
	}
}

// AddUser hashes the password with bcrypt, appends the user to the config, and saves it.
func AddUser(cfgPath, username, password string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check for duplicate
	for _, u := range cfg.Users {
		if u.Username == username {
			return fmt.Errorf("user %q already exists", username)
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	cfg.Users = append(cfg.Users, UserConfig{
		Username:     username,
		PasswordHash: string(hash),
	})

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("user %q added\n", username)
	return nil
}

// RemoveUser removes the user with the given username from the config and saves it.
func RemoveUser(cfgPath, username string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	found := false
	filtered := make([]UserConfig, 0, len(cfg.Users))
	for _, u := range cfg.Users {
		if u.Username == username {
			found = true
			continue
		}
		filtered = append(filtered, u)
	}

	if !found {
		return fmt.Errorf("user %q not found", username)
	}

	cfg.Users = filtered

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("user %q removed\n", username)
	return nil
}

// UpdateUser updates the password hash for an existing user and saves the config.
func UpdateUser(cfgPath, username, newPassword string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	found := false
	for i := range cfg.Users {
		if cfg.Users[i].Username == username {
			hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
			if err != nil {
				return fmt.Errorf("hash password: %w", err)
			}
			cfg.Users[i].PasswordHash = string(hash)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("user %q not found", username)
	}

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("user %q updated\n", username)
	return nil
}
