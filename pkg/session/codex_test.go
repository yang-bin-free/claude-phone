package session

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCodexProcBuildsNewAndResumeCommands(t *testing.T) {
	fresh := NewCodexProc(CodexConfig{
		Bin: "codex", Cwd: "/repo", Permission: "workspaceWrite", Model: "gpt-test",
		AddDirs: []string{"/shared"},
	})
	got, err := fresh.buildArgs("fix it")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"-C", "/repo", "-s", "workspace-write", "-a", "never", "-m", "gpt-test", "--add-dir", "/shared", "exec", "--json", "--color", "never", "--skip-git-repo-check", "fix it"} {
		if !slices.Contains(got, want) {
			t.Fatalf("fresh args %v missing %q", got, want)
		}
	}
	if slices.Contains(got, "resume") {
		t.Fatalf("fresh args unexpectedly resume: %v", got)
	}

	resumed := NewCodexProc(CodexConfig{
		Bin: "codex", Cwd: "/repo", ProviderSessionID: "thread-1", Permission: "readOnly",
	})
	got, err = resumed.buildArgs("continue")
	if err != nil {
		t.Fatal(err)
	}
	if !containsArgSequence(got, "exec", "resume") || !slices.Contains(got, "thread-1") || !slices.Contains(got, "read-only") {
		t.Fatalf("resume args = %v", got)
	}
	if slices.Contains(got, "--color") {
		t.Fatalf("resume args include unsupported --color flag: %v", got)
	}
}

func TestCodexProcMapsFullAccessAndRejectsUnknownPermission(t *testing.T) {
	full := NewCodexProc(CodexConfig{Cwd: "/repo", Permission: "fullAccess"})
	args, err := full.buildArgs("work")
	if err != nil || !slices.Contains(args, "danger-full-access") {
		t.Fatalf("full access args=%v err=%v", args, err)
	}
	unknown := NewCodexProc(CodexConfig{Cwd: "/repo", Permission: "default"})
	if _, err := unknown.buildArgs("work"); err == nil {
		t.Fatal("unknown Codex permission was accepted")
	}
}

func containsArgSequence(values []string, sequence ...string) bool {
	if len(sequence) == 0 || len(sequence) > len(values) {
		return false
	}
	for i := 0; i <= len(values)-len(sequence); i++ {
		if slices.Equal(values[i:i+len(sequence)], sequence) {
			return true
		}
	}
	return false
}

func TestCodexProcCapturesThreadBeforeOutputAndReleasesBeforeTerminalEvent(t *testing.T) {
	proc := NewCodexProc(CodexConfig{
		Bin: "../../testdata/fake-codex.sh", Cwd: ".", Permission: "readOnly",
	})
	terminal := make(chan error, 1)
	proc.OnOutput(func(payload []byte) {
		var event struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(payload, &event) != nil {
			return
		}
		if event.Type == "thread.started" && proc.ProviderSessionID() != "thread-fake" {
			terminal <- fmt.Errorf("thread ID was not captured before callback")
		}
		if event.Type == "turn.completed" {
			terminal <- proc.Send("second turn")
		}
	})
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	if err := proc.Send("first turn"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-terminal:
		if err != nil {
			t.Fatalf("terminal callback could not start next turn: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for terminal event")
	}
	_ = proc.Stop()
}

func TestCodexProcRejectsConcurrentSendAndStopSuppressesExitError(t *testing.T) {
	proc := NewCodexProc(CodexConfig{
		Bin: "../../testdata/fake-codex.sh", Cwd: ".", Permission: "readOnly",
	})
	var mu sync.Mutex
	var output []string
	proc.OnOutput(func(payload []byte) {
		mu.Lock()
		output = append(output, string(payload))
		mu.Unlock()
	})
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	if err := proc.Send("SLOW"); err != nil {
		t.Fatal(err)
	}
	if err := proc.Send("overlap"); err == nil {
		t.Fatal("concurrent Codex turn was accepted")
	}
	if err := proc.Stop(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	for _, line := range output {
		if strings.Contains(line, "CODEX_ERROR") {
			t.Fatalf("Stop emitted a synthetic error: %v", output)
		}
	}
}

func TestCodexProcReportsBoundedProcessFailure(t *testing.T) {
	proc := NewCodexProc(CodexConfig{
		Bin: "../../testdata/fake-codex.sh", Cwd: ".", Permission: "readOnly",
	})
	result := make(chan string, 2)
	proc.OnOutput(func(payload []byte) {
		result <- string(payload)
	})
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	if err := proc.Send("FAIL"); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if strings.Contains(got, "simulated Codex failure") || !strings.Contains(got, "exit status 7") || len(got) > 2300 {
			t.Fatalf("process error = %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Codex process error")
	}
	select {
	case got := <-result:
		if !strings.Contains(got, `"type":"done"`) {
			t.Fatalf("process failure did not finish turn: %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for Codex failure completion")
	}
	_ = proc.Stop()
}

func TestReadBoundedStderrDrainsInputAfterCaptureLimit(t *testing.T) {
	reader := strings.NewReader(strings.Repeat("x\n", codexMaxStderrBytes))
	got := readBoundedStderr(reader)
	if len(got) > codexMaxStderrBytes {
		t.Fatalf("captured stderr length=%d", len(got))
	}
	if reader.Len() != 0 {
		t.Fatalf("stderr reader left %d bytes undrained", reader.Len())
	}
}

func TestCodexProcOversizedOutputFinishesTurnWithoutLeakingOrDeadlocking(t *testing.T) {
	for _, prompt := range []string{"HUGE_STDERR", "HUGE_STDOUT"} {
		t.Run(prompt, func(t *testing.T) {
			proc := NewCodexProc(CodexConfig{Bin: "../../testdata/fake-codex.sh", Cwd: ".", Permission: "readOnly"})
			messages := make(chan string, 2)
			proc.OnOutput(func(payload []byte) {
				if strings.Contains(string(payload), `"type":"error"`) || strings.Contains(string(payload), `"type":"done"`) {
					messages <- string(payload)
				}
			})
			if err := proc.Start(); err != nil {
				t.Fatal(err)
			}
			if err := proc.Send(prompt); err != nil {
				t.Fatal(err)
			}
			for index := 0; index < 2; index++ {
				select {
				case message := <-messages:
					if strings.Contains(message, strings.Repeat("s", 32)) {
						t.Fatalf("raw stderr leaked: %.100q", message)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("%s deadlocked before terminal messages", prompt)
				}
			}
			if err := proc.Send("after oversized output"); err != nil {
				t.Fatalf("process did not release turn: %v", err)
			}
			_ = proc.Stop()
		})
	}
}
