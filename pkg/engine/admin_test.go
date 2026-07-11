package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestAdminHandlerPersistsAndDeletesProjects(t *testing.T) {
	dataDir := t.TempDir()
	projectDir := t.TempDir()
	e := New(Config{DataDir: dataDir})
	h := e.AdminHandler("secret")

	body := `{"name":"Demo","path":` + mustJSONString(t, projectDir) + `,"permission":"default"}`
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, adminRequest(http.MethodPost, "/admin/projects", body, "secret"))
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createW.Code, createW.Body.String())
	}
	var created adminproto.Project
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ProjectID == "" || created.Path != projectDir {
		t.Fatalf("created=%+v", created)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "projects.yaml")); err != nil {
		t.Fatalf("projects file: %v", err)
	}

	statusW := httptest.NewRecorder()
	h.ServeHTTP(statusW, adminRequest(http.MethodGet, "/admin/status", "", "secret"))
	var snapshot adminproto.Snapshot
	if err := json.NewDecoder(statusW.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Projects) != 1 || snapshot.Projects[0].Name != "Demo" {
		t.Fatalf("projects=%+v", snapshot.Projects)
	}

	deleteW := httptest.NewRecorder()
	h.ServeHTTP(deleteW, adminRequest(http.MethodDelete, "/admin/projects/"+created.ProjectID, "", "secret"))
	if deleteW.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleteW.Code, deleteW.Body.String())
	}
}

func TestAdminHandlerCreatesPersistentDeviceCredential(t *testing.T) {
	dataDir := t.TempDir()
	e := New(Config{DataDir: dataDir})
	h := e.AdminHandler("secret")
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, adminRequest(http.MethodPost, "/admin/devices", `{"name":"Pixel"}`, "secret"))
	if createW.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", createW.Code, createW.Body.String())
	}
	var credential adminproto.DeviceCredential
	if err := json.NewDecoder(createW.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(credential.DeviceToken, "dt_") || credential.Device.Name != "Pixel" {
		t.Fatalf("credential=%+v", credential)
	}

	restarted := New(Config{DataDir: dataDir})
	if !restarted.deviceAuthorized(credential.DeviceToken) {
		t.Fatal("persisted credential was not authorized after restart")
	}
}

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestAdminHandlerReturnsSnapshotAndStopsSession(t *testing.T) {
	e := New(Config{AgentVersion: "test", DeviceTokens: map[string]string{"dt_super_secret": "Pixel"}})
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
	if strings.Contains(statusW.Body.String(), "dt_super_secret") {
		t.Fatalf("snapshot leaked device token: %s", statusW.Body.String())
	}
	if len(snapshot.Devices) != 1 || snapshot.Devices[0].Name != "Pixel" {
		t.Fatalf("devices=%+v", snapshot.Devices)
	}
	if snapshot.Diagnostics.GoVersion == "" || snapshot.Diagnostics.GOOS == "" || snapshot.Diagnostics.DataDir == "" {
		t.Fatalf("diagnostics=%+v", snapshot.Diagnostics)
	}

	revokeReq := adminRequest(http.MethodDelete, "/admin/devices/"+snapshot.Devices[0].DeviceID, "", "secret")
	revokeW := httptest.NewRecorder()
	h.ServeHTTP(revokeW, revokeReq)
	if revokeW.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", revokeW.Code, revokeW.Body.String())
	}
	if _, ok := e.cfg.DeviceTokens["dt_super_secret"]; ok {
		t.Fatal("revoked token remains authorized")
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
