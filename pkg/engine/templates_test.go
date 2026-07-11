package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateStoreReadsLatestFile(t *testing.T) {
	dir := t.TempDir()
	store := newTemplateStore(dir)
	path := filepath.Join(dir, "templates.yaml")
	if err := os.WriteFile(path, []byte("templates:\n  - label: Test\n    prompt: Run tests\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err := store.List()
	if err != nil || len(items) != 1 || items[0].Prompt != "Run tests" {
		t.Fatalf("items=%v err=%v", items, err)
	}
	if err := os.WriteFile(path, []byte("templates:\n  - label: Review\n    prompt: Review code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	items, err = store.List()
	if err != nil || items[0].Label != "Review" {
		t.Fatalf("hot items=%v err=%v", items, err)
	}
}
