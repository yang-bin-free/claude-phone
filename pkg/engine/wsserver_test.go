package engine

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func TestWebSocket_ListProjectsUsesAdminProjectStore(t *testing.T) {
	e := New(Config{DataDir: t.TempDir()})
	projectDir := t.TempDir()
	if _, err := e.projects.Add(adminproto.Project{Name: "Demo", Path: projectDir, Permission: "default"}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(e.Handler())
	defer ts.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+ts.URL[len("http"):]+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	writeJSON(t, conn, protocol.AuthMsg{Type: protocol.TypeAuth, DeviceToken: "device", DeviceName: "Android"})
	assertType(t, conn, protocol.TypeHello)
	writeJSON(t, conn, protocol.ControlMsg{Type: protocol.TypeControl, Action: protocol.ActionListProjects})
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read projects: %v", err)
	}
	var msg protocol.ProjectListMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.TypeProjectList || len(msg.Projects) != 1 || msg.Projects[0].Path != projectDir {
		t.Fatalf("project list: %+v", msg)
	}
}

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

func TestWebSocket_StreamFanoutToTwoSubscribers(t *testing.T) {
	e := New(Config{
		AgentVersion:      "test",
		ClaudeVersion:     "fake",
		ClaudeBin:         "../../testdata/fake-claude.sh",
		DefaultWorkingDir: ".",
		DefaultPermission: "bypassPermissions",
		DeviceTokens:      map[string]string{"owner": "Mac", "guest": "Android"},
	})
	ts := httptest.NewServer(e.Handler())
	defer ts.Close()
	wsURL := "ws" + ts.URL[len("http"):] + "/ws"

	owner, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	writeJSON(t, owner, protocol.AuthMsg{Type: protocol.TypeAuth, DeviceToken: "owner", DeviceName: "Mac"})
	assertType(t, owner, protocol.TypeHello)
	writeJSON(t, owner, protocol.ControlMsg{Type: protocol.TypeControl, Action: protocol.ActionCreateSession, Name: "shared"})
	created := readSessionCreated(t, owner)

	guest, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer guest.Close()
	writeJSON(t, guest, protocol.AuthMsg{Type: protocol.TypeAuth, DeviceToken: "guest", DeviceName: "Android"})
	assertType(t, guest, protocol.TypeHello)
	writeJSON(t, guest, protocol.ControlMsg{Type: protocol.TypeControl, Action: protocol.ActionJoinSession, SessionID: created.SessionID})
	writeJSON(t, guest, protocol.ControlMsg{Type: protocol.TypeControl, Action: protocol.ActionListSessions, Limit: 10})
	assertType(t, guest, protocol.TypeSessionList)

	writeJSON(t, owner, protocol.TextMsg{Type: protocol.TypeText, Content: "fanout"})
	want := []string{protocol.TypeThinking, protocol.TypeToken, protocol.TypeToken, protocol.TypeDone}
	for _, typ := range want {
		assertType(t, owner, typ)
		assertType(t, guest, typ)
	}
}

func TestRemoveClientDoesNotRemoveReplacementConnection(t *testing.T) {
	e := New(Config{})
	oldClient := &client{deviceID: "same-device", mu: make(chan struct{}, 1)}
	newClient := &client{deviceID: "same-device", mu: make(chan struct{}, 1)}
	e.addClient(oldClient)
	e.addClient(newClient)

	e.removeClient(oldClient)

	e.mu.RLock()
	got := e.clients["same-device"]
	e.mu.RUnlock()
	if got != newClient {
		t.Fatalf("replacement connection was removed: got %p want %p", got, newClient)
	}
}

func readSessionCreated(t *testing.T, conn *websocket.Conn) protocol.SessionCreatedMsg {
	t.Helper()
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg protocol.SessionCreatedMsg
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if msg.Type != protocol.TypeSessionCreated || msg.SessionID == "" {
		t.Fatalf("unexpected message: %s", payload)
	}
	return msg
}

func TestHandleControl_LeaveSession(t *testing.T) {
	e := New(Config{})
	e.manager = session.NewManager(session.ManagerConfig{
		MaxConcurrent: 5,
		IDFunc:        func() string { return "sess-1" },
		Now:           func() int64 { return 1 },
	})
	s, err := e.manager.Create("车险联调", "/p", "default", "device-A")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	s.Subscribe("device-B")

	cl := &client{deviceID: "device-B", mu: make(chan struct{}, 1)}
	next, err := e.handleControl(cl, s.ID, mustControl(t, protocol.ControlMsg{
		Type:      protocol.TypeControl,
		Action:    protocol.ActionLeaveSession,
		SessionID: s.ID,
	}))
	if err != nil {
		t.Fatalf("leave: %v", err)
	}
	if next != "" {
		t.Fatalf("current session not cleared: %q", next)
	}
	subs := s.Subscribers()
	if len(subs) != 1 || subs[0] != "device-A" {
		t.Fatalf("unexpected subscribers: %v", subs)
	}
	if s.Status != "active" {
		t.Fatalf("status=%q", s.Status)
	}
}

func TestHandleControl_StopSession(t *testing.T) {
	e := New(Config{})
	e.manager = session.NewManager(session.ManagerConfig{
		MaxConcurrent: 5,
		IDFunc:        func() string { return "sess-1" },
		Now:           func() int64 { return 1 },
	})
	s, err := e.manager.Create("车险联调", "/p", "default", "device-A")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	proc := &stubClaudeProc{}
	e.procs[s.ID] = proc

	cl := &client{deviceID: "device-A", mu: make(chan struct{}, 1)}
	next, err := e.handleControl(cl, s.ID, mustControl(t, protocol.ControlMsg{
		Type:      protocol.TypeControl,
		Action:    protocol.ActionStopSession,
		SessionID: s.ID,
	}))
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if next != "" {
		t.Fatalf("current session not cleared: %q", next)
	}
	if proc.stopCalls != 1 {
		t.Fatalf("stop calls=%d", proc.stopCalls)
	}
	if _, ok := e.manager.Get(s.ID); ok {
		t.Fatalf("session still present after stop")
	}
	if s.Status != "stopped" {
		t.Fatalf("status=%q", s.Status)
	}
}

func TestSessionListPagination(t *testing.T) {
	nextID := 0
	var now int64
	e := New(Config{})
	e.manager = session.NewManager(session.ManagerConfig{
		MaxConcurrent: 5,
		IDFunc: func() string {
			nextID++
			return fmt.Sprintf("sess-%d", nextID)
		},
		Now: func() int64 {
			now++
			return now
		},
	})
	s1, _ := e.manager.Create("a", "/a", "default", "device-A")
	s2, _ := e.manager.Create("b", "/b", "default", "device-A")
	s3, _ := e.manager.Create("c", "/c", "default", "device-A")

	msg := e.sessionList(1, 1)
	if len(msg.Sessions) != 1 {
		t.Fatalf("sessions len=%d", len(msg.Sessions))
	}
	if msg.Sessions[0].SessionID != s2.ID {
		t.Fatalf("got %q, want %q", msg.Sessions[0].SessionID, s2.ID)
	}
	if s1.ID == s3.ID {
		t.Fatalf("ids should differ")
	}
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

func mustControl(t *testing.T, v protocol.ControlMsg) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal control: %v", err)
	}
	return b
}

type stubClaudeProc struct {
	stopCalls int
}

func (p *stubClaudeProc) OnOutput(session.OutputFunc) {}
func (p *stubClaudeProc) Start() error                { return nil }
func (p *stubClaudeProc) Send(string) error           { return nil }
func (p *stubClaudeProc) Stop() error {
	p.stopCalls++
	return nil
}
