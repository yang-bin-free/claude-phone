package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsMigrateLegacyDataDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	legacy := filepath.Join(home, ".claude-phone")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "devices.yaml"), []byte("devices: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := (Config{}).withDefaults()
	want := filepath.Join(home, ".codeafar")
	if got.DataDir != want {
		t.Fatalf("DataDir=%q want %q", got.DataDir, want)
	}
	if _, err := os.Stat(filepath.Join(want, "devices.yaml")); err != nil {
		t.Fatalf("legacy data was not migrated: %v", err)
	}
}

func TestConfigDefaultsKeepExplicitDataDirectory(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "custom")
	got := (Config{DataDir: explicit}).withDefaults()
	if got.DataDir != explicit {
		t.Fatalf("DataDir=%q want %q", got.DataDir, explicit)
	}
}
