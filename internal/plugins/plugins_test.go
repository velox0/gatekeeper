package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnabledReadsAssets(t *testing.T) {
	dir := t.TempDir()
	name := "testplugin"

	// Create plugin directory with CSS and JS files.
	pluginDir := filepath.Join(dir, name)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	cssContent := "body { background: red; }"
	jsContent := "console.log('hello');"

	if err := os.WriteFile(filepath.Join(pluginDir, name+".css"), []byte(cssContent), 0644); err != nil {
		t.Fatalf("WriteFile CSS error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, name+".js"), []byte(jsContent), 0644); err != nil {
		t.Fatalf("WriteFile JS error: %v", err)
	}

	// Override home dir for this test by using loadEnabledFrom helper.
	loaded, err := loadEnabledFrom(dir, map[string]bool{name: true})
	if err != nil {
		t.Fatalf("loadEnabledFrom error: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("loaded = %d plugins, want 1", len(loaded))
	}
	if loaded[0].Name != name {
		t.Fatalf("plugin name = %q, want %q", loaded[0].Name, name)
	}
	if string(loaded[0].CSS) != cssContent {
		t.Fatalf("CSS = %q, want %q", loaded[0].CSS, cssContent)
	}
	if string(loaded[0].JS) != jsContent {
		t.Fatalf("JS = %q, want %q", loaded[0].JS, jsContent)
	}
}

func TestLoadEnabledSkipsDisabledPlugins(t *testing.T) {
	dir := t.TempDir()
	loaded, err := loadEnabledFrom(dir, map[string]bool{"something": false})
	if err != nil {
		t.Fatalf("loadEnabledFrom error: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded = %d plugins, want 0", len(loaded))
	}
}

func TestLoadEnabledSkipsMissingAssets(t *testing.T) {
	dir := t.TempDir()
	// Plugin enabled but no assets at all.
	loaded, err := loadEnabledFrom(dir, map[string]bool{"ghost": true})
	if err != nil {
		t.Fatalf("loadEnabledFrom error: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded = %d plugins, want 0 (no assets)", len(loaded))
	}
}

func TestRenderCSSWrapsInStyleTag(t *testing.T) {
	p := LoadedPlugin{CSS: "body{}"}
	got := string(p.RenderCSS())
	if got != "<style>\nbody{}\n</style>" {
		t.Fatalf("RenderCSS = %q", got)
	}
}

func TestRenderJSWrapsInScriptTag(t *testing.T) {
	p := LoadedPlugin{JS: "alert(1)"}
	got := string(p.RenderJS())
	if got != "<script>\nalert(1)\n</script>" {
		t.Fatalf("RenderJS = %q", got)
	}
}

func TestRenderCSSEmptyReturnsEmpty(t *testing.T) {
	p := LoadedPlugin{}
	if got := p.RenderCSS(); got != "" {
		t.Fatalf("RenderCSS should be empty, got %q", got)
	}
}

func TestRenderJSEmptyReturnsEmpty(t *testing.T) {
	p := LoadedPlugin{}
	if got := p.RenderJS(); got != "" {
		t.Fatalf("RenderJS should be empty, got %q", got)
	}
}
