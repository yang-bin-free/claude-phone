package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func TestHandlerServesShellAssetsAndDelegatesAPIs(t *testing.T) {
	engineHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ws" {
			t.Fatalf("engine path=%q", r.URL.Path)
		}
		w.WriteHeader(http.StatusSwitchingProtocols)
	})
	adminHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/status" {
			t.Fatalf("admin path=%q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"agent":{}}`))
	})
	status := AppStatus{Ready: true, ClaudeBin: "/usr/local/bin/claude", ClaudeVersion: "1.2.3"}
	h := NewHandler(HandlerOptions{
		EngineHandler: func() http.Handler { return engineHandler },
		AdminHandler:  func() http.Handler { return adminHandler },
		Status:        func() AppStatus { return status },
	})

	tests := []struct {
		path        string
		wantStatus  int
		wantContent string
	}{
		{path: "/", wantStatus: http.StatusOK, wantContent: "text/html"},
		{path: "/assets/provider-workspace.js", wantStatus: http.StatusOK, wantContent: "text/javascript"},
		{path: "/assets/chat.js", wantStatus: http.StatusOK, wantContent: "text/javascript"},
		{path: "/assets/core.css", wantStatus: http.StatusOK, wantContent: "text/css"},
		{path: "/desktop/status", wantStatus: http.StatusOK, wantContent: "application/json"},
		{path: "/ws", wantStatus: http.StatusSwitchingProtocols},
		{path: "/admin/status", wantStatus: http.StatusOK, wantContent: "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if w.Code != tt.wantStatus {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if tt.wantContent != "" && w.Header().Get("Content-Type") != tt.wantContent {
				t.Fatalf("content-type=%q", w.Header().Get("Content-Type"))
			}
		})
	}
}

func TestDesktopProjectEndpointRejectsNonLoopbackRequest(t *testing.T) {
	h := NewHandler(HandlerOptions{AddProject: func(string) (protocol.ProjectInfo, error) {
		t.Fatal("non-loopback request reached project callback")
		return protocol.ProjectInfo{}, nil
	}})
	request := httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":"/tmp"}`))
	request.RemoteAddr = "100.64.0.2:1234"
	response := httptest.NewRecorder()

	h.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestDesktopProjectEndpointAddsPickedDirectory(t *testing.T) {
	directory := t.TempDir()
	h := NewHandler(HandlerOptions{AdminToken: "secret", AddProject: func(path string) (protocol.ProjectInfo, error) {
		if path != directory {
			t.Fatalf("path=%q want %q", path, directory)
		}
		return protocol.ProjectInfo{Name: "Demo", Path: path, Permission: "default"}, nil
	}})
	request := httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":`+quotedJSON(t, directory)+`}`))
	request.RemoteAddr = "127.0.0.1:1234"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CodeAfar-Admin-Token", "secret")
	response := httptest.NewRecorder()

	h.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got protocol.ProjectInfo
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != directory || got.Name != "Demo" {
		t.Fatalf("project=%+v", got)
	}
}

func TestDesktopProjectEndpointRejectsLoopbackCSRF(t *testing.T) {
	called := false
	h := NewHandler(HandlerOptions{AdminToken: "secret", AddProject: func(string) (protocol.ProjectInfo, error) {
		called = true
		return protocol.ProjectInfo{}, nil
	}})
	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":"/tmp"}`)),
		httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":"/tmp"}`)),
		httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":"/tmp"}`)),
	}
	requests[0].Header.Set("Content-Type", "text/plain")
	requests[0].Header.Set("X-CodeAfar-Admin-Token", "secret")
	requests[1].Header.Set("Content-Type", "application/json")
	requests[2].Header.Set("Content-Type", "application/json")
	requests[2].Header.Set("X-CodeAfar-Admin-Token", "wrong")
	for _, request := range requests {
		request.RemoteAddr = "127.0.0.1:1234"
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if called {
		t.Fatal("unauthorized loopback request reached project callback")
	}
}

func quotedJSON(t *testing.T, value string) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHandlerKeepsShellAvailableWithoutEngine(t *testing.T) {
	h := NewHandler(HandlerOptions{Status: func() AppStatus {
		return AppStatus{Error: "Claude CLI not found"}
	}})

	for _, path := range []string{"/", "/desktop/status"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/ws", "/admin/status"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d body=%s", path, w.Code, w.Body.String())
		}
	}
}
