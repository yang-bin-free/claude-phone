package engine

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func TestStatusEndpoint(t *testing.T) {
	e := New(Config{
		Addr:                 "127.0.0.1:9876",
		AgentVersion:         "test",
		ClaudeVersion:        "fake",
		ClaudeBin:            "claude",
		DefaultWorkingDir:    "/work",
		DefaultPermission:    "bypassPermissions",
		MaxConcurrentSession: 3,
	})
	e.manager = session.NewManager(session.ManagerConfig{
		MaxConcurrent: 3,
		IDFunc:        func() string { return "sess-1" },
		Now:           func() int64 { return 123 },
	})
	s, err := e.manager.Create("demo", "/work", "bypassPermissions", "device-A")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	e.mu.Lock()
	e.procs[s.ID] = &stubStatusProc{}
	e.mu.Unlock()
	e.mu.Lock()
	e.clients["device-A"] = &client{deviceID: "device-A", mu: make(chan struct{}, 1)}
	e.mu.Unlock()

	ts := httptest.NewServer(e.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer resp.Body.Close()
	var report StatusReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.AgentVersion != "test" || report.ClaudeVersion != "fake" {
		t.Fatalf("report mismatch: %+v", report)
	}
	if len(report.ConnectedDevices) != 1 || report.ConnectedDevices[0] != "device-A" {
		t.Fatalf("connected devices: %+v", report.ConnectedDevices)
	}
	if len(report.Sessions) != 1 || report.Sessions[0].SessionID != s.ID {
		t.Fatalf("sessions: %+v", report.Sessions)
	}
	if !report.Sessions[0].Running {
		t.Fatalf("expected running session snapshot")
	}
}

type stubStatusProc struct{}

func (stubStatusProc) OnOutput(session.OutputFunc) {}
func (stubStatusProc) Start() error                { return nil }
func (stubStatusProc) Send(string) error           { return nil }
func (stubStatusProc) Stop() error                 { return nil }
