package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const defaultConfigContent = `# ─── Global scope ───
app_name: Gatekeeper
auth:
  cookie_name: gatekeeper_session
  session_ttl: 24h0m0s
security:
  secure_cookies: true
  same_site: strict
  authorize_favicon: false
users: []
plugins:
  aurora: false
  hearts: true
  matrix: false
  winxp: false

# ─── Listeners & Virtual Hosts ───
listeners:
  - listen: ":8080"
    servers:
      - server_name: ""
        upstream:
          target: http://localhost:3000
`

// HandleConfigCommand handles the config subcommand CLI.
func HandleConfigCommand(args []string) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printConfigHelp()
		os.Exit(0)
	}

	subcmd := args[0]
	switch subcmd {
	case "init":
		initConfig()
	case "service":
		printServiceConfig()
	default:
		fmt.Fprintf(os.Stderr, "unknown config subcommand: %s\nRun 'gatekeeper config help' for usage.\n", subcmd)
		os.Exit(1)
	}
}

func initConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error determining home directory: %v\n", err)
		os.Exit(1)
	}

	dir := filepath.Join(home, ".gatekeeper")
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", dir, err)
		os.Exit(1)
	}

	path := filepath.Join(dir, "config.yml")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(os.Stderr, "config file already exists at %s\n", path)
		os.Exit(1)
	}

	if err := os.WriteFile(path, []byte(defaultConfigContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing config file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Initialized default configuration at %s\n", path)
}

func printServiceConfig() {
	switch runtime.GOOS {
	case "linux":
		fmt.Println("=== Systemd Service Configuration (Linux) ===")
		fmt.Println("Save the following content to /etc/systemd/system/gatekeeper.service:")
		fmt.Println()
		fmt.Println(`[Unit]
Description=Gatekeeper Authentication Gateway
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/gatekeeper -config /etc/gatekeeper/config.yml -pid /var/run/gatekeeper.pid
Restart=on-failure
RestartSec=5s
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target`)
		fmt.Println()
		fmt.Println("=== Installation Steps ===")
		fmt.Println("1. Copy the gatekeeper binary to /usr/local/bin:")
		fmt.Println("   sudo cp gatekeeper /usr/local/bin/")
		fmt.Println()
		fmt.Println("2. Create config directory and copy your configuration:")
		fmt.Println("   sudo mkdir -p /etc/gatekeeper")
		fmt.Println("   sudo cp ~/.gatekeeper/config.yml /etc/gatekeeper/config.yml")
		fmt.Println()
		fmt.Println("3. Reload systemd, enable and start the service:")
		fmt.Println("   sudo systemctl daemon-reload")
		fmt.Println("   sudo systemctl enable gatekeeper")
		fmt.Println("   sudo systemctl start gatekeeper")

	case "darwin":
		fmt.Println("=== Launchd Plist Configuration (macOS) ===")
		fmt.Println("Save the following content to /Library/LaunchDaemons/com.velox0.gatekeeper.plist:")
		fmt.Println()
		fmt.Println(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.velox0.gatekeeper</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/gatekeeper</string>
        <string>-config</string>
        <string>/Library/Application Support/Gatekeeper/config.yml</string>
        <string>-pid</string>
        <string>/var/run/gatekeeper.pid</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/var/log/gatekeeper.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/gatekeeper.err.log</string>
</dict>
</plist>`)
		fmt.Println()
		fmt.Println("=== Installation Steps ===")
		fmt.Println("1. Copy the gatekeeper binary to /usr/local/bin:")
		fmt.Println("   sudo cp gatekeeper /usr/local/bin/")
		fmt.Println()
		fmt.Println("2. Create configuration directory and copy config:")
		fmt.Println("   sudo mkdir -p \"/Library/Application Support/Gatekeeper\"")
		fmt.Println("   sudo cp ~/.gatekeeper/config.yml \"/Library/Application Support/Gatekeeper/config.yml\"")
		fmt.Println()
		fmt.Println("3. Load the daemon:")
		fmt.Println("   sudo launchctl load -w /Library/LaunchDaemons/com.velox0.gatekeeper.plist")

	default:
		fmt.Printf("=== Background Service Instructions (%s) ===\n", runtime.GOOS)
		fmt.Println("1. Compile the gatekeeper binary.")
		fmt.Println("2. Run the binary as a background daemon or service using your OS process manager (e.g. NSSM on Windows, daemon tools, supervisor, or cron @reboot).")
	}
}

// printConfigHelp prints the config usage help.
func printConfigHelp() {
	fmt.Println("Gatekeeper - Configuration Management")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gatekeeper config init             Initialize default configuration at ~/.gatekeeper/config.yml")
	fmt.Println("  gatekeeper config service          Generate background service configuration for your OS")
	fmt.Println("  gatekeeper config help             Show this help message")
}
