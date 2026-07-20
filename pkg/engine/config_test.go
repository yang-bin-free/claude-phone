package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/provider"
)

func TestConfigDefaultsDoNotHideMigrationErrors(t *testing.T) {
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
	if _, err := os.Stat(filepath.Join(legacy, "devices.yaml")); err != nil {
		t.Fatalf("withDefaults unexpectedly moved legacy data: %v", err)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("withDefaults unexpectedly created data directory: %v", err)
	}
}

func TestDefaultRegistryContainsClaudeAndCodexDescriptors(t *testing.T) {
	e := New(Config{
		DataDir: t.TempDir(), ClaudeBin: "claude", CodexBin: "codex",
		CodexUnavailableReason: "codex missing",
	})
	defer e.Close()
	claude, claudeOK := e.providers.Get(provider.ClaudeID)
	codex, codexOK := e.providers.Get(provider.CodexID)
	if !claudeOK || !claude.Descriptor().Available {
		t.Fatalf("Claude descriptor=%+v ok=%v", claude, claudeOK)
	}
	if !codexOK || codex.Descriptor().Available || codex.Descriptor().UnavailableReason != "codex missing" {
		t.Fatalf("Codex descriptor=%+v ok=%v", codex, codexOK)
	}
}

func TestConfigDefaultsKeepExplicitDataDirectory(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "custom")
	got := (Config{DataDir: explicit}).withDefaults()
	if got.DataDir != explicit {
		t.Fatalf("DataDir=%q want %q", got.DataDir, explicit)
	}
}
