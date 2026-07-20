package engine

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

// This test is opt-in because it uses the developer's authenticated Claude CLI
// and therefore makes a real model request. Release QA runs it explicitly.
func TestRealClaudeEndToEnd(t *testing.T) {
	if os.Getenv("CODEAFAR_REAL_CLAUDE") != "1" {
		t.Skip("set CODEAFAR_REAL_CLAUDE=1 to use the authenticated Claude CLI")
	}
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	e := New(Config{
		DataDir:           t.TempDir(),
		ClaudeBin:         claudeBin,
		DefaultWorkingDir: repo,
		DefaultPermission: "default",
		DeviceTokens:      map[string]string{"real-device": "Release QA"},
	})
	defer e.Close()
	server, conn := openAuthenticatedEngine(t, e, "real-device")
	defer server.Close()
	defer conn.Close()

	writeJSON(t, conn, protocol.ControlMsg{
		Type: protocol.TypeControl, Action: protocol.ActionCreateSession,
		Name: "CodeAfar real E2E", WorkingDir: repo, Provider: "claude",
		PermissionMode: "default", RequestID: "release-real-e2e",
	})
	created := readSessionCreated(t, conn)
	if created.Provider != "claude" || created.Cwd != repo {
		t.Fatalf("created=%+v", created)
	}
	writeJSON(t, conn, protocol.TextMsg{
		Type:    protocol.TypeText,
		Content: "不要使用任何工具，只回复精确文本 CODEAFAR_E2E_OK",
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	var response strings.Builder
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var envelope protocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		switch envelope.Type {
		case protocol.TypeToken:
			var token protocol.TokenMsg
			if err := json.Unmarshal(payload, &token); err != nil {
				t.Fatal(err)
			}
			response.WriteString(token.Content)
		case protocol.TypeError:
			var message protocol.ErrorMsg
			_ = json.Unmarshal(payload, &message)
			t.Fatalf("Claude error: %+v", message)
		case protocol.TypeDone:
			if !strings.Contains(response.String(), "CODEAFAR_E2E_OK") {
				t.Fatalf("unexpected Claude response %q", response.String())
			}
			return
		}
	}
}

func TestRealClaudeToolInputEndToEnd(t *testing.T) {
	if os.Getenv("CODEAFAR_REAL_CLAUDE") != "1" {
		t.Skip("set CODEAFAR_REAL_CLAUDE=1 to use the authenticated Claude CLI")
	}
	claudeBin, err := exec.LookPath("claude")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	e := New(Config{
		DataDir:           t.TempDir(),
		ClaudeBin:         claudeBin,
		DefaultWorkingDir: repo,
		DefaultPermission: "default",
		DeviceTokens:      map[string]string{"real-device": "Release QA"},
	})
	defer e.Close()
	server, conn := openAuthenticatedEngine(t, e, "real-device")
	defer server.Close()
	defer conn.Close()

	writeJSON(t, conn, protocol.ControlMsg{
		Type: protocol.TypeControl, Action: protocol.ActionCreateSession,
		Name: "CodeAfar tool input E2E", WorkingDir: repo, Provider: "claude",
		PermissionMode: "default", RequestID: "release-real-tool-e2e",
	})
	_ = readSessionCreated(t, conn)
	writeJSON(t, conn, protocol.TextMsg{
		Type:    protocol.TypeText,
		Content: "必须使用 Read 工具读取 README.md 的第一行，然后只回复这一行。",
	})

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	sawCompleteRead := false
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		var envelope protocol.Envelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatal(err)
		}
		switch envelope.Type {
		case protocol.TypeToolUse:
			var message protocol.ToolUseMsg
			if err := json.Unmarshal(payload, &message); err != nil {
				t.Fatal(err)
			}
			if message.Tool != "Read" {
				continue
			}
			var input struct {
				FilePath string `json:"file_path"`
			}
			if err := json.Unmarshal([]byte(message.Input), &input); err != nil {
				t.Fatalf("Read input is not complete JSON: %q: %v", message.Input, err)
			}
			if filepath.Base(input.FilePath) != "README.md" {
				t.Fatalf("Read input lost file path: %q", message.Input)
			}
			sawCompleteRead = true
		case protocol.TypeError:
			var message protocol.ErrorMsg
			_ = json.Unmarshal(payload, &message)
			t.Fatalf("Claude error: %+v", message)
		case protocol.TypeDone:
			if !sawCompleteRead {
				t.Fatal("Claude completed without a Read call containing its full input")
			}
			return
		}
	}
}
