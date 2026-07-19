package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeSessionExists reports whether Claude Code has a resumable transcript
// for this session in the requested working directory.
func ClaudeSessionExists(cwd, sessionID string) bool {
	if sessionID == "" || filepath.Base(sessionID) != sessionID || strings.ContainsAny(sessionID, "*?[") {
		return false
	}
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false
		}
		root = filepath.Join(home, ".claude")
	}
	target, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(root, "projects", "*", sessionID+".jsonl"))
	if err != nil {
		return false
	}
	for _, path := range matches {
		if transcriptHasCWD(path, target) {
			return true
		}
	}
	return false
}

func transcriptHasCWD(path, target string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Cwd == "" {
			continue
		}
		actual, err := filepath.Abs(event.Cwd)
		if err == nil && filepath.Clean(actual) == filepath.Clean(target) {
			return true
		}
	}
	return false
}
