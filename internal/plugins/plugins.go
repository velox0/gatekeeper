// Package plugins manages login-page plugin assets stored at ~/.gatekeeper/<name>/.
// Default plugin assets are embedded in the binary and seeded on first run.
package plugins

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

//go:embed assets
var defaultAssets embed.FS

const gatekeeperDir = ".gatekeeper"

// PluginDir returns the resolved path to ~/.gatekeeper.
func PluginDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, gatekeeperDir), nil
}

// PopulateDefaults copies the embedded default plugin assets into ~/.gatekeeper/<name>/
// if they don't already exist. Existing files are never overwritten.
func PopulateDefaults() error {
	pluginDir, err := PluginDir()
	if err != nil {
		return err
	}

	return fs.WalkDir(defaultAssets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute the relative path after "assets/"
		rel, err := filepath.Rel("assets", path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dest := filepath.Join(pluginDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		// Skip if the file already exists on disk
		if _, err := os.Stat(dest); err == nil {
			return nil
		}

		data, err := defaultAssets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}

		log.Printf("seeding default plugin asset: %s", dest)
		return os.WriteFile(dest, data, 0644)
	})
}

// LoadedPlugin holds the CSS and JS content for an enabled plugin.
type LoadedPlugin struct {
	Name string
	CSS  template.CSS
	JS   template.JS
}

// LoadEnabled reads CSS and JS files from ~/.gatekeeper/<name>/ for each
// enabled plugin. Missing files are silently skipped (a plugin may have
// only CSS or only JS).
func LoadEnabled(enabled map[string]bool) ([]LoadedPlugin, error) {
	pluginDir, err := PluginDir()
	if err != nil {
		return nil, err
	}

	var loaded []LoadedPlugin
	for name, on := range enabled {
		if !on {
			continue
		}

		dir := filepath.Join(pluginDir, name)
		cssFile := filepath.Join(dir, name+".css")
		jsFile := filepath.Join(dir, name+".js")

		var cssData, jsData []byte

		if b, err := os.ReadFile(cssFile); err == nil {
			cssData = b
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", cssFile, err)
		}

		if b, err := os.ReadFile(jsFile); err == nil {
			jsData = b
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", jsFile, err)
		}

		if len(cssData) == 0 && len(jsData) == 0 {
			log.Printf("warning: plugin %q enabled but no assets found at %s", name, dir)
			continue
		}

		loaded = append(loaded, LoadedPlugin{
			Name: name,
			CSS:  template.CSS(cssData),
			JS:   template.JS(jsData),
		})
	}

	return loaded, nil
}
