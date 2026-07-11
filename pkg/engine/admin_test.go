package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func TestAdminHandlerRejectsUnauthorizedOrRemoteRequests(t *testing.T) {
	e := New(Config{})
	h := e.AdminHandler("secret")

	tests := []struct {
		name       string
		remoteAddr string
		token      string
	}{
		{name: "missing token", remoteAddr: "127.0.0.1:5000"},
		{name: "wrong token", remoteAddr: "127.0.0.1:5000", token: "wrong"},
		{name: "remote peer", remoteAddr: "100.64.0.2:5000", token: "secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusForbidden {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAdminHandlerReturnsSnapshotAndStopsSession(t *testing.T) {
	e := New(Config{AgentVersion: "test"})
	e.manager = session.NewManager(session.ManagerConfig{IDFunc: func() string { return "sess-1" }})
	s, err := e.manager.Create("demo", "/work", "default", "device-A")
	if err != nil {
		t.Fatal(err)
	}
	e.procs[s.ID] = &stubAdminProc{}
	h := e.AdminHandler("secret")

	statusReq := adminRequest(http.MethodGet, "/admin/status", "", "secret")
	statusW := httptest.NewRecorder()
	h.ServeHTTP(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", statusW.Code, statusW.Body.String())
	}
	var snapshot adminproto.Snapshot
	if err := json.NewDecoder(statusW.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Agent.AgentVersion != "test" || len(snapshot.Agent.Sessions) != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}

	stopReq := adminRequest(http.MethodPost, "/admin/sessions/stop", `{"sessionId":"sess-1"}`, "secret")
	stopW := httptest.NewRecorder()
	h.ServeHTTP(stopW, stopReq)
	if stopW.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", stopW.Code, stopW.Body.String())
	}
	if _, ok := e.manager.Get(s.ID); ok {
		t.Fatal("session was not removed")
	}
}

func adminRequest(method, target, body, token string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:5000"
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

type stubAdminProc struct{}

func (stubAdminProc) OnOutput(session.OutputFunc) {}
func (stubAdminProc) Start() error                { return nil }
func (stubAdminProc) Send(string) error           { return nil }
func (stubAdminProc) Stop() error                 { return nil }
