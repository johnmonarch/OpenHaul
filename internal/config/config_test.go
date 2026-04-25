package config

import (
	"os"
	"path/filepath"
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
