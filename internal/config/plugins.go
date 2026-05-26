package config

import (
	"fmt"
	"os"
	"sort"

	"github.com/velox0/gatekeeper/internal/daemon"
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
}

// RegisterPlugin adds a plugin to the global registry.
func RegisterPlugin(name, description string) {
	registry = append(registry, PluginInfo{Name: name, Description: description})
}

// KnownPlugins returns a sorted copy of all registered plugins.
func KnownPlugins() []PluginInfo {
	out := make([]PluginInfo, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// IsKnownPlugin checks whether a plugin name is registered.
func IsKnownPlugin(name string) bool {
	for _, p := range registry {
		if p.Name == name {
			return true
		}
	}
	return false
}

// HandlePluginsCommand parses and dispatches plugin management subcommands.
// Usage:
//
//	gatekeeper plugin list
//	gatekeeper plugin enable  <name>
//	gatekeeper plugin disable <name>
func HandlePluginsCommand(cfgPath, pidPath string, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: gatekeeper plugin <list|enable|disable> ...")
		os.Exit(1)
	}

	subcmd := args[0]
	rest := args[1:]

	switch subcmd {
	case "list":
		listPlugins(cfgPath)
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
		fmt.Fprintf(os.Stderr, "unknown plugin subcommand: %s\n", subcmd)
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
