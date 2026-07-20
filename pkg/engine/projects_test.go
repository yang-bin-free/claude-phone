package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddProjectAuthorizesExistingDirectory(t *testing.T) {
	e := New(Config{DataDir: t.TempDir()})
	directory := t.TempDir()

	project, err := e.AddProject(directory)
	if err != nil {
		t.Fatal(err)
	}
	if project.Path != directory || project.Name != filepath.Base(directory) || project.Permission != "default" {
		t.Fatalf("project=%+v", project)
	}
	projects, err := e.projects.List()
	if err != nil || len(projects) != 1 || projects[0].Path != directory {
		t.Fatalf("projects=%+v err=%v", projects, err)
	}
}

func TestAddProjectRejectsFile(t *testing.T) {
	e := New(Config{DataDir: t.TempDir()})
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddProject(file); err == nil {
		t.Fatal("file was accepted as project")
	}
}
