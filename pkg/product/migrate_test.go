package product

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDirMigratesLegacyDirectory(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, LegacyDataDirName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "projects.yaml"), []byte("projects: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, migrated, err := ResolveDataDir(home, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, DataDirName)
	if got != want || !migrated {
		t.Fatalf("ResolveDataDir() = %q, %v; want %q, true", got, migrated, want)
	}
	if _, err := os.Stat(filepath.Join(got, "projects.yaml")); err != nil {
		t.Fatalf("migrated file: %v", err)
	}
	if _, err := os.Stat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy directory still exists: %v", err)
	}
}

func TestResolveDataDirDoesNotOverwriteExistingDestination(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, LegacyDataDirName)
	current := filepath.Join(home, DataDirName)
	for _, path := range []string{legacy, current} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(current, "marker"), []byte("current"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, migrated, err := ResolveDataDir(home, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != current || migrated {
		t.Fatalf("ResolveDataDir() = %q, %v; want %q, false", got, migrated, current)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory was changed: %v", err)
	}
}

func TestResolveDataDirLeavesExplicitPathUntouched(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, LegacyDataDirName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(home, "custom")

	got, migrated, err := ResolveDataDir(home, explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit || migrated {
		t.Fatalf("ResolveDataDir() = %q, %v; want %q, false", got, migrated, explicit)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("legacy directory was changed: %v", err)
	}
}

func TestResolveDataDirReturnsMigrationFailure(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, LegacyDataDirName)
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	if _, _, err := ResolveDataDir(home, ""); err == nil {
		t.Fatal("expected migration error instead of a fresh data directory")
	}
}
