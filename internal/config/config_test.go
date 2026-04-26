package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExpandsMirrorLocalPathHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OHG_HOME", "")
	t.Setenv("OHG_CONFIG", "")
	t.Setenv("OHG_DB_PATH", "")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	body := []byte(`
db_path = "/tmp/openhaulguard-test.db"

[sources]
[sources.mirror]
enabled = true
local_path = "~/.openhaulguard/mirror/carriers.json"
`)
	if err := os.WriteFile(configPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Overrides{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	want := filepath.Join(home, ".openhaulguard", "mirror", "carriers.json")
	if cfg.Sources.Mirror.LocalPath != want {
		t.Fatalf("mirror local path = %q, want %q", cfg.Sources.Mirror.LocalPath, want)
	}
}

func TestDefaultDoesNotConfigureHostedMirrorURL(t *testing.T) {
	home := t.TempDir()
	cfg, err := Default(Overrides{Home: home})
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
	if cfg.Sources.Mirror.URL != "" || cfg.Sources.Mirror.ChecksumURL != "" {
		t.Fatalf("mirror URLs = %q / %q, want empty until a hosted mirror exists", cfg.Sources.Mirror.URL, cfg.Sources.Mirror.ChecksumURL)
	}
}

func TestSaveOmitsEmptyHostedMirrorURL(t *testing.T) {
	home := t.TempDir()
	cfg, err := Default(Overrides{Home: home})
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	body, err := os.ReadFile(cfg.Path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	text := string(body)
	if strings.Contains(text, "url =") || strings.Contains(text, "checksum_url =") {
		t.Fatalf("config should not include empty hosted mirror URL fields:\n%s", text)
	}
}
