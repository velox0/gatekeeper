package config

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/velox0/gatekeeper/internal/daemon"
)

// maxBcryptPasswordLen is the maximum password length bcrypt supports.
// Passwords longer than this are silently truncated, weakening security.
const maxBcryptPasswordLen = 72

// HandleUserCommand parses and dispatches user management subcommands.
// Usage:
//
//	gatekeeper user add <username>
//	gatekeeper user remove <username>
//	gatekeeper user update <username>
func HandleUserCommand(cfgPath, pidPath string, args []string) {
	cfgPath = ResolveConfigPath(cfgPath, pidPath)

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUserHelp()
		os.Exit(0)
	}

	subcmd := args[0]
	rest := args[1:]

	var err error
	switch subcmd {
	case "add":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper user add <username>")
			os.Exit(1)
		}
		password, pErr := readPasswordInteractively()
		if pErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", pErr)
			os.Exit(1)
		}
		err = AddUser(cfgPath, rest[0], password)
	case "remove":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper user remove <username>")
			os.Exit(1)
		}
		err = RemoveUser(cfgPath, rest[0])
	case "update":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper user update <username>")
			os.Exit(1)
		}
		password, pErr := readPasswordInteractively()
		if pErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", pErr)
			os.Exit(1)
		}
		err = UpdateUser(cfgPath, rest[0], password)
	default:
		fmt.Fprintf(os.Stderr, "unknown user subcommand: %s\nRun 'gatekeeper user help' for usage.\n", subcmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Signal the running daemon to reload config
	if sigErr := daemon.SignalReload(pidPath); sigErr != nil {
		fmt.Printf("warning: config saved but could not signal daemon: %v\n", sigErr)
		fmt.Println("the daemon will pick up changes on next restart")
	} else {
		fmt.Println("config saved and daemon signaled to reload")
	}
}

// AddUser hashes the password with bcrypt and adds the user to the selected scope(s).
func AddUser(cfgPath, username, password string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	refs := ListServerBlocks(cfg)
	globalSelected, selectedRefs := PromptServerBlockSelection(refs, true)

	if len(password) > maxBcryptPasswordLen {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxBcryptPasswordLen)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	newUser := UserConfig{
		Username:     username,
		PasswordHash: string(hash),
	}

	if globalSelected {
		// Check for duplicate in global scope
		for _, u := range cfg.Users {
			if u.Username == username {
				return fmt.Errorf("user %q already exists in global scope", username)
			}
		}
		cfg.Users = append(cfg.Users, newUser)
		fmt.Printf("user %q added to global scope\n", username)
	}

	for _, ref := range selectedRefs {
		srv := &cfg.Listeners[ref.ListenerIdx].Servers[ref.ServerIdx]
		// Check for duplicate in this server block
		for _, u := range srv.Users {
			if u.Username == username {
				srvName := ref.ServerName
				if srvName == "" {
					srvName = "<default>"
				}
				return fmt.Errorf("user %q already exists in server block %s → %s", username, ref.Listen, srvName)
			}
		}
		srv.Users = append(srv.Users, newUser)
		srvName := ref.ServerName
		if srvName == "" {
			srvName = "<default>"
		}
		fmt.Printf("user %q added to %s → %s\n", username, ref.Listen, srvName)
	}

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

// RemoveUser removes the user from the selected scope(s).
func RemoveUser(cfgPath, username string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	refs := ListServerBlocks(cfg)
	globalSelected, selectedRefs := PromptServerBlockSelection(refs, true)

	if globalSelected {
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
			return fmt.Errorf("user %q not found in global scope", username)
		}
		cfg.Users = filtered
		fmt.Printf("user %q removed from global scope\n", username)
	}

	for _, ref := range selectedRefs {
		srv := &cfg.Listeners[ref.ListenerIdx].Servers[ref.ServerIdx]
		found := false
		filtered := make([]UserConfig, 0, len(srv.Users))
		for _, u := range srv.Users {
			if u.Username == username {
				found = true
				continue
			}
			filtered = append(filtered, u)
		}
		srvName := ref.ServerName
		if srvName == "" {
			srvName = "<default>"
		}
		if !found {
			return fmt.Errorf("user %q not found in %s → %s", username, ref.Listen, srvName)
		}
		srv.Users = filtered
		fmt.Printf("user %q removed from %s → %s\n", username, ref.Listen, srvName)
	}

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

// UpdateUser updates the password hash for an existing user in the selected scope(s).
func UpdateUser(cfgPath, username, newPassword string) error {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	refs := ListServerBlocks(cfg)
	globalSelected, selectedRefs := PromptServerBlockSelection(refs, true)

	if len(newPassword) > maxBcryptPasswordLen {
		return fmt.Errorf("password exceeds maximum length of %d bytes", maxBcryptPasswordLen)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if globalSelected {
		found := false
		for i := range cfg.Users {
			if cfg.Users[i].Username == username {
				cfg.Users[i].PasswordHash = string(hash)
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %q not found in global scope", username)
		}
		fmt.Printf("user %q updated in global scope\n", username)
	}

	for _, ref := range selectedRefs {
		srv := &cfg.Listeners[ref.ListenerIdx].Servers[ref.ServerIdx]
		found := false
		for i := range srv.Users {
			if srv.Users[i].Username == username {
				srv.Users[i].PasswordHash = string(hash)
				found = true
				break
			}
		}
		srvName := ref.ServerName
		if srvName == "" {
			srvName = "<default>"
		}
		if !found {
			return fmt.Errorf("user %q not found in %s → %s", username, ref.Listen, srvName)
		}
		fmt.Printf("user %q updated in %s → %s\n", username, ref.Listen, srvName)
	}

	if err := SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func readPasswordInteractively() (string, error) {
	fmt.Print("Enter password: ")
	p1, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	fmt.Print("Confirm password: ")
	p2, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read confirmation: %w", err)
	}

	if string(p1) != string(p2) {
		return "", fmt.Errorf("passwords do not match")
	}

	if len(p1) == 0 {
		return "", fmt.Errorf("password cannot be empty")
	}

	return string(p1), nil
}

// printUserHelp displays help details for user commands.
func printUserHelp() {
	fmt.Println("Gatekeeper - User Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gatekeeper user add <username>     Add a new user (prompts securely for password)")
	fmt.Println("  gatekeeper user remove <username>  Remove an existing user")
	fmt.Println("  gatekeeper user update <username>  Update a user's password")
	fmt.Println("  gatekeeper user help               Show this help message")
}
