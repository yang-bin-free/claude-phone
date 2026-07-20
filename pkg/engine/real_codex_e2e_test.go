package engine

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/provider"
)

// This test is opt-in because it uses the developer's authenticated Codex CLI
// and therefore makes real model requests. Release QA runs it explicitly.
func TestRealCodexMultiTurnResumeEndToEnd(t *testing.T) {
	if os.Getenv("CODEAFAR_REAL_CODEX") != "1" {
		t.Skip("set CODEAFAR_REAL_CODEX=1 to use the authenticated Codex CLI")
	}
	codexBin, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	config := Config{
		DataDir: dataDir, CodexBin: codexBin, ClaudeUnavailableReason: "not used in Codex E2E",
		DefaultWorkingDir: repo, DeviceTokens: map[string]string{"real-codex-device": "Release QA"},
	}

	e := New(config)
	server, conn := openAuthenticatedEngine(t, e, "real-codex-device")
	writeJSON(t, conn, protocol.ControlMsg{
		Type: protocol.TypeControl, Action: protocol.ActionCreateSession,
		Name: "CodeAfar Codex real E2E", WorkingDir: repo, Provider: provider.CodexID,
		PermissionMode: "workspaceWrite", RequestID: "real-codex-create",
	})
	created := readSessionCreated(t, conn)
	writeJSON(t, conn, protocol.TextMsg{
		Type:    protocol.TypeText,
		Content: "Use the shell to run printf CODEAFAR_CODEX_TOOL_OK. Then reply with CODEAFAR_CODEX_TURN1 and remember the secret BLUE-PINE-731.",
	})
	waitForCodexTurn(t, conn, "CODEAFAR_CODEX_TURN1", true)

	writeJSON(t, conn, protocol.TextMsg{
		Type:    protocol.TypeText,
		Content: "Without using tools, reply with the secret from the previous turn and CODEAFAR_CODEX_TURN2.",
	})
	waitForCodexTurn(t, conn, "BLUE-PINE-731", false)

	sess, ok := e.manager.Get(created.SessionID)
	if !ok || sess.ProviderSessionIdentity() == "" {
		t.Fatalf("Codex thread identity was not persisted: session=%+v", sess)
	}
	_ = conn.Close()
	server.Close()
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}

	restarted := New(config)
	defer restarted.Close()
	server, conn = openAuthenticatedEngine(t, restarted, "real-codex-device")
	defer server.Close()
	defer conn.Close()
	writeJSON(t, conn, protocol.ControlMsg{
		Type: protocol.TypeControl, Action: protocol.ActionSelectSession, SessionID: created.SessionID,
	})
	writeJSON(t, conn, protocol.TextMsg{
		Type:    protocol.TypeText,
		Content: "Without using tools, reply with the remembered secret and CODEAFAR_CODEX_RESTART_OK.",
	})
	waitForCodexTurn(t, conn, "CODEAFAR_CODEX_RESTART_OK", false)
}

func waitForCodexTurn(t *testing.T, conn *websocket.Conn, marker string, requireTool bool) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
	var response strings.Builder
	var errorsSeen []string
	sawTool := false
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %s: %v; response=%q errors=%v", marker, err, response.String(), errorsSeen)
		}
		var envelope protocol.Envelope
		if json.Unmarshal(payload, &envelope) != nil {
			continue
		}
		switch envelope.Type {
		case protocol.TypeToken:
			var token protocol.TokenMsg
			_ = json.Unmarshal(payload, &token)
			response.WriteString(token.Content)
		case protocol.TypeToolUse:
			var tool protocol.ToolUseMsg
			_ = json.Unmarshal(payload, &tool)
			if tool.Tool == "Bash" {
				sawTool = true
			}
		case protocol.TypeError:
			var message protocol.ErrorMsg
			_ = json.Unmarshal(payload, &message)
			errorsSeen = append(errorsSeen, message.Message)
		case protocol.TypeDone:
			if !strings.Contains(response.String(), marker) {
				t.Fatalf("Codex response missing %q: response=%q errors=%v", marker, response.String(), errorsSeen)
			}
			if requireTool && !sawTool {
				t.Fatalf("Codex completed without a translated Bash tool event: response=%q", response.String())
			}
			return
		}
	}
}
