package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
	h := NewHandler(engineHandler, adminHandler)

	tests := []struct {
		path        string
		wantStatus  int
		wantContent string
	}{
		{path: "/", wantStatus: http.StatusOK, wantContent: "text/html"},
		{path: "/assets/chat.js", wantStatus: http.StatusOK, wantContent: "text/javascript"},
		{path: "/assets/core.css", wantStatus: http.StatusOK, wantContent: "text/css"},
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
