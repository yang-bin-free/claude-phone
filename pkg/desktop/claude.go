package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveClaudeBinary finds a runnable Claude CLI even when the app was
// launched from Finder with macOS's minimal PATH.
func ResolveClaudeBinary(requested string) (string, error) {
	return resolveCodingAgentBinary(requested, "claude", "Claude", true)
}

// ResolveCodexBinary finds Codex when Finder launches CodeAfar with a minimal PATH.
func ResolveCodexBinary(requested string) (string, error) {
	return resolveCodingAgentBinary(requested, "codex", "Codex", false)
}

func resolveCodingAgentBinary(requested, defaultName, displayName string, includeClaudeLocal bool) (string, error) {
	if strings.TrimSpace(requested) == "" {
		requested = defaultName
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
		return "", fmt.Errorf("%s CLI is not executable; searched: %s", displayName, strings.Join(searched, ", "))
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
	candidates := []string{
		filepath.Join(home, ".local", "bin", requested),
		filepath.Join(home, ".volta", "bin", requested),
		filepath.Join(home, ".asdf", "shims", requested),
		filepath.Join(home, ".local", "share", "mise", "shims", requested),
	}
	if includeClaudeLocal {
		candidates = append([]string{filepath.Join(home, ".claude", "local", requested)}, candidates...)
	}
	nvmPattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", requested)
	nvmCandidates, _ := filepath.Glob(nvmPattern)
	sort.Sort(sort.Reverse(sort.StringSlice(nvmCandidates)))
	if len(nvmCandidates) == 0 {
		candidates = append(candidates, nvmPattern)
	} else {
		candidates = append(candidates, nvmCandidates...)
	}
	candidates = append(candidates,
		"/opt/homebrew/bin/"+requested,
		"/usr/local/bin/"+requested,
	)
	for _, path := range candidates {
		add(path)
		if executableFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("%s CLI was not found; searched: %s", displayName, strings.Join(searched, ", "))
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
