package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeSessionExistsMatchesTranscriptCWD(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "4e2858dd-c712-4f0e-9818-c05191acf107"
	dir := filepath.Join(root, "projects", "encoded-project")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := []byte("{\"type\":\"system\"}\n{\"type\":\"system\",\"cwd\":\"/work\"}\n")
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), transcript, 0o600); err != nil {
		t.Fatal(err)
	}

	if !ClaudeSessionExists("/work", id) {
		t.Fatal("existing transcript was not found")
	}
	if ClaudeSessionExists("/other", id) {
		t.Fatal("transcript from another cwd matched")
	}
}

func TestClaudeSessionExistsResolvesRelativeWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "80b52889-3dc9-4dea-80b5-de3d1a51a216"
	dir := filepath.Join(root, "projects", "root")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := []byte("{\"type\":\"system\",\"cwd\":\"/\"}\n")
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), transcript, 0o600); err != nil {
		t.Fatal(err)
	}

	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("/"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	if !ClaudeSessionExists(".", id) {
		t.Fatal("relative working directory did not resolve to transcript cwd")
	}
}

func TestClaudeSessionExistsReturnsFalseForMissingOrUnsafeID(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if ClaudeSessionExists("/work", "missing") {
		t.Fatal("missing transcript matched")
	}
	if ClaudeSessionExists("/work", "*") {
		t.Fatal("glob metacharacter was accepted")
	}
}
