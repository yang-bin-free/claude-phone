package session

import (
	"slices"
	"sync"
	"testing"
	"time"
)

func TestClaudeProcIncludesAllowedTools(t *testing.T) {
	args := NewClaudeProc(ClaudeConfig{SessionID: "s", Permission: "default", AllowedTools: []string{"Read", "Bash(git status:*)"}}).buildArgs()
	if !slices.Contains(args, "--allowedTools") || !slices.Contains(args, "Bash(git status:*)") {
		t.Fatalf("args missing allowed tools: %v", args)
	}
	if !slices.Contains(args, "--include-partial-messages") {
		t.Fatalf("args missing partial message streaming: %v", args)
	}
}

func TestClaudeProc_StreamsTokens(t *testing.T) {
	proc := NewClaudeProc(ClaudeConfig{
		Bin:        "../../testdata/fake-claude.sh",
		Cwd:        ".",
		SessionID:  "sess1",
		Permission: "bypassPermissions",
	})

	var mu sync.Mutex
	var lines []string
	proc.OnOutput(func(payload []byte) {
		mu.Lock()
		lines = append(lines, string(payload))
		mu.Unlock()
	})

	if err := proc.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := proc.Send("检查并发"); err != nil {
		t.Fatalf("send: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := len(lines)
		mu.Unlock()
		if n >= 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout, got %d lines: %v", n, lines)
		case <-time.After(20 * time.Millisecond):
		}
	}
	_ = proc.Stop()

	if lines[0] != `{"type":"thinking"}` {
		t.Fatalf("first line = %s", lines[0])
	}
	if lines[len(lines)-1] != `{"type":"done"}` {
		t.Fatalf("last line = %s", lines[len(lines)-1])
	}
}
