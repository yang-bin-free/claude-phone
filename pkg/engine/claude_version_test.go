package engine

import "testing"

func TestClaudeVersionPattern(t *testing.T) {
	for input, want := range map[string]string{"2.1.204 (Claude Code)": "2.1.204", "claude version 3.0.0-beta.1": "3.0.0-beta.1"} {
		match := claudeVersionPattern.FindStringSubmatch(input)
		if len(match) != 2 || match[1] != want {
			t.Fatalf("parse %q = %v, want %q", input, match, want)
		}
	}
}

func TestCLIVersionPatternSupportsCodex(t *testing.T) {
	match := cliVersionPattern.FindStringSubmatch("codex-cli 0.144.1")
	if len(match) != 2 || match[1] != "0.144.1" {
		t.Fatalf("match=%v", match)
	}
}
