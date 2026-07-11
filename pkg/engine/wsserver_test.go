package engine

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func TestWebSocket_CreateSessionAndStream(t *testing.T) {
	e := New(Config{
		AgentVersion:      "test",
		ClaudeVersion:     "fake",
		ClaudeBin:         "../../testdata/fake-claude.sh",
		DefaultWorkingDir: ".",
		DefaultPermission: "bypassPermissions",
		DeviceTokens:      map[string]string{"dt_abc": "Pixel"},
	})
	ts := httptest.NewServer(e.Handler())
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, protocol.AuthMsg{Type: protocol.TypeAuth, DeviceToken: "dt_abc", DeviceName: "Pixel"})
	assertType(t, conn, protocol.TypeHello)

	writeJSON(t, conn, protocol.ControlMsg{
		Type:           protocol.TypeControl,
		Action:         protocol.ActionCreateSession,
		Name:           "车险联调",
		WorkingDir:     ".",
		PermissionMode: "bypassPermissions",
	})
	assertType(t, conn, protocol.TypeSessionCreated)

	writeJSON(t, conn, protocol.TextMsg{Type: protocol.TypeText, Content: "检查并发"})
	want := []string{protocol.TypeThinking, protocol.TypeToken, protocol.TypeToken, protocol.TypeDone}
	for _, typ := range want {
		assertType(t, conn, typ)
	}
}

func TestWebSocket_RejectsUnknownDevice(t *testing.T) {
	e := New(Config{DeviceTokens: map[string]string{"dt_ok": "Pixel"}})
	ts := httptest.NewServer(e.Handler())
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	writeJSON(t, conn, protocol.AuthMsg{Type: protocol.TypeAuth, DeviceToken: "bad"})
	assertType(t, conn, protocol.TypeError)
}

func writeJSON(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	if err := conn.WriteJSON(v); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func assertType(t *testing.T, conn *websocket.Conn, want string) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read %s: %v", want, err)
	}
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &head); err != nil {
		t.Fatalf("unmarshal %q: %v", payload, err)
	}
	if head.Type != want {
		t.Fatalf("got message %s, want type %s", payload, want)
	}
}
