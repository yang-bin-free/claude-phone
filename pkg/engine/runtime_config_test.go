package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRuntimeConfigReloadsSafeFields(t *testing.T) {
	dir := t.TempDir()
	e := New(Config{DataDir: dir, DefaultWorkingDir: "/old", DefaultPermission: "default", MaxConcurrentSession: 5, ConfigPollInterval: 10 * time.Millisecond})
	defer e.Close()

	config := []byte("defaultWorkingDir: /new\ndefaultPermission: acceptEdits\nmaxConcurrentSessions: 2\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), config, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got := e.Status()
		if got.DefaultWorkingDir == "/new" && got.DefaultPermission == "acceptEdits" && got.MaxConcurrentSession == 2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("config did not reload: %+v", e.Status())
}

func TestInvalidRuntimeConfigKeepsLastValidValues(t *testing.T) {
	dir := t.TempDir()
	e := New(Config{DataDir: dir, DefaultWorkingDir: "/old", DefaultPermission: "default", MaxConcurrentSession: 3})
	defer e.Close()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("maxConcurrentSessions: nope\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.reloadRuntimeConfig(); err == nil {
		t.Fatal("expected invalid YAML value to fail")
	}
	got := e.Status()
	if got.DefaultWorkingDir != "/old" || got.DefaultPermission != "default" || got.MaxConcurrentSession != 3 {
		t.Fatalf("invalid config changed runtime: %+v", got)
	}
}
