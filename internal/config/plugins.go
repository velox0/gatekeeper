package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/velox0/gatekeeper/internal/daemon"
	"github.com/velox0/gatekeeper/internal/plugins"
)

// PluginInfo describes a registered login-page plugin.
type PluginInfo struct {
	Name        string
	Description string
}

// registry is the global set of known plugins.
var registry []PluginInfo

func init() {
	// Built-in plugins are registered here.
	RegisterPlugin("hearts", "Floating heart emoji burst on page load")
	RegisterPlugin("matrix", "Matrix-style digital rain background")
	RegisterPlugin("aurora", "Animated northern-lights color bands")
	RegisterPlugin("winxp", "Windows XP Welcome Screen theme")
}

// RegisterPlugin adds a plugin to the global registry.
func RegisterPlugin(name, description string) {
	registry = append(registry, PluginInfo{Name: name, Description: description})
}

var scanOnce sync.Once

func scanCustomPlugins() {
	pluginDir, err := plugins.PluginDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Check if already in registry directly to avoid re-entry deadlocks
		found := false
		for _, p := range registry {
			if p.Name == name {
				found = true
				break
			}
		}
		if found {
			continue
		}
		// Check if it contains name.css or name.js
		cssPath := filepath.Join(pluginDir, name, name+".css")
		jsPath := filepath.Join(pluginDir, name, name+".js")
		_, errCSS := os.Stat(cssPath)
		_, errJS := os.Stat(jsPath)
		if errCSS == nil || errJS == nil {
			RegisterPlugin(name, "Custom plugin loaded from disk")
		}
	}
}

// KnownPlugins returns a sorted copy of all registered plugins.
func KnownPlugins() []PluginInfo {
	scanOnce.Do(scanCustomPlugins)
	out := make([]PluginInfo, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsKnownPlugin checks whether a plugin name is registered.
func IsKnownPlugin(name string) bool {
	scanOnce.Do(scanCustomPlugins)
	for _, p := range registry {
		if p.Name == name {
			return true
		}
	}
	return false
}

// HandlePluginCommand parses and dispatches plugin management subcommands.
// Usage:
//
//	gatekeeper plugin list
//	gatekeeper plugin enable  <name>
//	gatekeeper plugin disable <name>
func HandlePluginCommand(cfgPath, pidPath string, args []string) {
	cfgPath = ResolveConfigPath(cfgPath, pidPath)

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printPluginHelp()
		os.Exit(0)
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "list":
		listPlugins(cfgPath)
	case "add":
		if len(rest) != 2 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin add <name> <path/to/plugin>")
			os.Exit(1)
		}
		addPlugin(rest[0], rest[1])
	case "update":
		if len(rest) == 0 {
			updateDefaults("")
		} else if len(rest) == 1 {
			name := rest[0]
			if isDefaultPlugin(name) {
				updateDefaults(name)
			} else {
				fmt.Fprintf(os.Stderr, "error: %q is a custom plugin; to update it, please provide the path: gatekeeper plugin update <name> <path>\n", name)
				os.Exit(1)
			}
		} else if len(rest) == 2 {
			// Update custom plugin from path
			addPlugin(rest[0], rest[1])
		} else {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin update [<name> [<path>]]")
			os.Exit(1)
		}
	case "delete":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin delete <name>")
			os.Exit(1)
		}
		deletePlugin(cfgPath, pidPath, rest[0])
	case "enable":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin enable <name>")
			os.Exit(1)
		}
		setPlugin(cfgPath, pidPath, rest[0], true)
	case "disable":
		if len(rest) != 1 {
			fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin disable <name>")
			os.Exit(1)
		}
		setPlugin(cfgPath, pidPath, rest[0], false)
	default:
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\nRun 'gatekeeper plugin help' for usage.\n", subcmd)
		os.Exit(1)
	}
}

func listPlugins(cfgPath string) {
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	plugins := KnownPlugins()

	// Show global plugin state
	fmt.Println("=== Global Plugins ===")
	fmt.Printf("%-12s %-8s %s\n", "PLUGIN", "STATUS", "DESCRIPTION")
	for _, p := range plugins {
		status := "disabled"
		if cfg.Plugins[p.Name] {
			status = "enabled"
		}
		fmt.Printf("%-12s %-8s %s\n", p.Name, status, p.Description)
	}

	// Show per-server-block overrides
	for _, ln := range cfg.Listeners {
		for _, srv := range ln.Servers {
			if len(srv.Plugins) == 0 {
				continue
			}
			displayName := srv.ServerName
			if displayName == "" {
				displayName = "<default>"
			}
			fmt.Printf("\n=== %s → %s (overrides) ===\n", ln.Listen, displayName)
			fmt.Printf("%-12s %-8s\n", "PLUGIN", "STATUS")
			for _, p := range plugins {
				if v, ok := srv.Plugins[p.Name]; ok {
					status := "disabled"
					if v {
						status = "enabled"
					}
					fmt.Printf("%-12s %-8s\n", p.Name, status)
				}
			}
		}
	}
}

func setPlugin(cfgPath, pidPath, name string, enabled bool) {
	if !IsKnownPlugin(name) {
		fmt.Fprintf(os.Stderr, "error: unknown plugin %q\n", name)
		fmt.Fprintln(os.Stderr, "available plugins:")
		for _, p := range KnownPlugins() {
			fmt.Fprintf(os.Stderr, "  %-12s %s\n", p.Name, p.Description)
		}
		os.Exit(1)
	}

	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	refs := ListServerBlocks(cfg)
	globalSelected, selectedRefs := PromptServerBlockSelection(refs, true)

	if globalSelected {
		if cfg.Plugins == nil {
			cfg.Plugins = make(map[string]bool)
		}
		cfg.Plugins[name] = enabled
		verb := "disabled"
		if enabled {
			verb = "enabled"
		}
		fmt.Printf("plugin %q %s in global scope\n", name, verb)
	}

	for _, ref := range selectedRefs {
		srv := &cfg.Listeners[ref.ListenerIdx].Servers[ref.ServerIdx]
		if srv.Plugins == nil {
			srv.Plugins = make(map[string]bool)
		}
		srv.Plugins[name] = enabled
		verb := "disabled"
		if enabled {
			verb = "enabled"
		}
		srvName := ref.ServerName
		if srvName == "" {
			srvName = "<default>"
		}
		fmt.Printf("plugin %q %s in %s → %s\n", name, verb, ref.Listen, srvName)
	}

	if err := SaveConfig(cfg, cfgPath); err != nil {
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

// printPluginHelp displays help details for plugin commands.
func printPluginHelp() {
	fmt.Println("Gatekeeper - Visual Login Plugins")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gatekeeper plugin list                      List all visual login plugins and their statuses")
	fmt.Println("  gatekeeper plugin add <name> <path>         Copy and register a custom plugin from disk")
	fmt.Println("  gatekeeper plugin update [<name> [<path>]]  Update default built-in or custom plugins")
	fmt.Println("  gatekeeper plugin delete <name>             Delete a custom plugin's assets and config")
	fmt.Println("  gatekeeper plugin enable <name>             Enable a visual plugin for specific scopes")
	fmt.Println("  gatekeeper plugin disable <name>            Disable a visual plugin for specific scopes")
	fmt.Println("  gatekeeper plugin help                      Show this help message")
}

func isDefaultPlugin(name string) bool {
	defaults := map[string]bool{
		"hearts": true,
		"matrix": true,
		"aurora": true,
		"winxp":  true,
	}
	return defaults[name]
}

func updateDefaults(name string) {
	if err := plugins.UpdateDefaults(name); err != nil {
		fmt.Fprintf(os.Stderr, "error updating default plugins: %v\n", err)
		os.Exit(1)
	}
	if name == "" {
		fmt.Println("Successfully updated all default plugins from embedded assets!")
	} else {
		fmt.Printf("Successfully updated default plugin %q from embedded assets!\n", name)
	}
}

func deletePlugin(cfgPath, pidPath, name string) {
	if isDefaultPlugin(name) {
		fmt.Fprintf(os.Stderr, "error: %q is a built-in default plugin and cannot be deleted\n", name)
		fmt.Fprintln(os.Stderr, "use 'gatekeeper plugin disable' to turn it off instead")
		os.Exit(1)
	}

	// 1. Remove the asset directory
	pluginDir, err := plugins.PluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	dir := filepath.Join(pluginDir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: plugin %q not found at %s\n", name, dir)
		os.Exit(1)
	}
	if err := os.RemoveAll(dir); err != nil {
		fmt.Fprintf(os.Stderr, "error removing %s: %v\n", dir, err)
		os.Exit(1)
	}
	fmt.Printf("Removed plugin assets at %s\n", dir)

	// 2. Strip the plugin from all config scopes
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load config to clean up references: %v\n", err)
		return
	}

	changed := false
	if _, ok := cfg.Plugins[name]; ok {
		delete(cfg.Plugins, name)
		changed = true
	}
	for li := range cfg.Listeners {
		for si := range cfg.Listeners[li].Servers {
			if _, ok := cfg.Listeners[li].Servers[si].Plugins[name]; ok {
				delete(cfg.Listeners[li].Servers[si].Plugins, name)
				changed = true
			}
		}
	}

	if changed {
		if err := SaveConfig(cfg, cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Cleaned up plugin references from config")

		if sigErr := daemon.SignalReload(pidPath); sigErr != nil {
			fmt.Printf("warning: config saved but could not signal daemon: %v\n", sigErr)
		} else {
			fmt.Println("Config saved and daemon signaled to reload")
		}
	}

	fmt.Printf("Successfully deleted plugin %q\n", name)
}

func addPlugin(name, srcPath string) {
	// 1. Resolve destination directory ~/.gatekeeper/<name>
	pluginDir, err := plugins.PluginDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	destDir := filepath.Join(pluginDir, name)

	// 2. Read source directory
	entries, err := os.ReadDir(srcPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading source path %s: %v\n", srcPath, err)
		os.Exit(1)
	}

	// 3. Find candidate CSS and JS files
	var cssSrc, jsSrc string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		base := entry.Name()
		switch ext {
		case ".css":
			// Prioritize exact match name.css
			if base == name+".css" {
				cssSrc = filepath.Join(srcPath, entry.Name())
			} else if cssSrc == "" {
				cssSrc = filepath.Join(srcPath, entry.Name())
			}
		case ".js":
			// Prioritize exact match name.js
			if base == name+".js" {
				jsSrc = filepath.Join(srcPath, entry.Name())
			} else if jsSrc == "" {
				jsSrc = filepath.Join(srcPath, entry.Name())
			}
		}
	}

	if cssSrc == "" && jsSrc == "" {
		fmt.Fprintf(os.Stderr, "error: no .css or .js files found in %s\n", srcPath)
		os.Exit(1)
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory %s: %v\n", destDir, err)
		os.Exit(1)
	}

	// Copy files
	if cssSrc != "" {
		data, err := os.ReadFile(cssSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", cssSrc, err)
			os.Exit(1)
		}
		destFile := filepath.Join(destDir, name+".css")
		if err := os.WriteFile(destFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", destFile, err)
			os.Exit(1)
		}
		fmt.Printf("Copied CSS to %s\n", destFile)
	}

	if jsSrc != "" {
		data, err := os.ReadFile(jsSrc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading %s: %v\n", jsSrc, err)
			os.Exit(1)
		}
		destFile := filepath.Join(destDir, name+".js")
		if err := os.WriteFile(destFile, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing %s: %v\n", destFile, err)
			os.Exit(1)
		}
		fmt.Printf("Copied JS to %s\n", destFile)
	}

	// Dynamically register the newly added plugin
	RegisterPlugin(name, "Custom plugin added from disk")
	fmt.Printf("Successfully registered custom plugin %q!\n", name)
}
