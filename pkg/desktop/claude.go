package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveClaudeBinary finds a runnable Claude CLI even when the app was
// launched from Finder with macOS's minimal PATH.
func ResolveClaudeBinary(requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		requested = "claude"
	}
	searched := make([]string, 0, 6)
	seen := map[string]bool{}
	add := func(path string) {
		if path != "" && !seen[path] {
			seen[path] = true
			searched = append(searched, path)
		}
	}

	if strings.ContainsRune(requested, filepath.Separator) {
		path, err := filepath.Abs(requested)
		if err == nil {
			add(path)
			if executableFile(path) {
				return path, nil
			}
		}
		return "", fmt.Errorf("Claude CLI is not executable; searched: %s", strings.Join(searched, ", "))
	}

	if path, err := exec.LookPath(requested); err == nil {
		if absolute, absErr := filepath.Abs(path); absErr == nil {
			path = absolute
		}
		add(path)
		if executableFile(path) {
			return path, nil
		}
	}

	home, _ := os.UserHomeDir()
	for _, path := range []string{
		filepath.Join(home, ".local", "bin", requested),
		filepath.Join(home, ".claude", "local", requested),
		"/opt/homebrew/bin/" + requested,
		"/usr/local/bin/" + requested,
	} {
		add(path)
		if executableFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("Claude CLI was not found; searched: %s", strings.Join(searched, ", "))
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
