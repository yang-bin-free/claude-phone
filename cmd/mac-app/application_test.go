package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/desktop"
	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

type fakeManagedEngine struct {
	closed atomic.Bool
}

func (e *fakeManagedEngine) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusSwitchingProtocols) })
}

func (e *fakeManagedEngine) AdminHandler(string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

func (e *fakeManagedEngine) Close() error {
	e.closed.Store(true)
	return nil
}

func (e *fakeManagedEngine) Status() engine.StatusReport {
	return engine.StatusReport{ConnectedDevices: []string{"Mac"}, Sessions: []engine.SessionSnapshot{{SessionID: "s1"}}}
}

func TestApplicationKeepsDesktopAliveWhenClaudeIsUnavailable(t *testing.T) {
	app := newApplication(context.Background(), appConfig{DesktopAddr: "127.0.0.1:0", ClaudeBin: "claude", AdminToken: "token"}, appDependencies{
		resolveClaude: func(string) (string, error) { return "", errors.New("claude missing") },
	})
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	status := app.Status()
	if status.Ready || !strings.Contains(status.Error, "claude missing") {
		t.Fatalf("status=%+v", status)
	}
	assertAppHTTPStatus(t, app.Handler(), "/", http.StatusOK)
	assertAppHTTPStatus(t, app.Handler(), "/desktop/status", http.StatusOK)
	assertAppHTTPStatus(t, app.Handler(), "/ws", http.StatusServiceUnavailable)
}

func TestApplicationPauseAndResumeKeepsDesktopHandler(t *testing.T) {
	created := make([]*fakeManagedEngine, 0, 2)
	app := newApplication(context.Background(), appConfig{DesktopAddr: "127.0.0.1:0", ClaudeBin: "claude", AdminToken: "token"}, appDependencies{
		resolveClaude: func(string) (string, error) { return "/tmp/claude", nil },
		detectVersion: func(string) (string, error) { return "1.2.3", nil },
		newEngine: func(engine.Config) managedEngine {
			instance := &fakeManagedEngine{}
			created = append(created, instance)
			return instance
		},
	})
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	if !app.Status().Ready {
		t.Fatalf("status=%+v", app.Status())
	}
	assertAppHTTPStatus(t, app.Handler(), "/ws", http.StatusSwitchingProtocols)

	if err := app.Pause(); err != nil {
		t.Fatal(err)
	}
	if !created[0].closed.Load() || !app.Status().Paused {
		t.Fatalf("closed=%v status=%+v", created[0].closed.Load(), app.Status())
	}
	assertAppHTTPStatus(t, app.Handler(), "/ws", http.StatusServiceUnavailable)
	assertAppHTTPStatus(t, app.Handler(), "/", http.StatusOK)

	if err := app.Resume(); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 || !app.Status().Ready {
		t.Fatalf("created=%d status=%+v", len(created), app.Status())
	}
}

func TestApplicationCloseIsIdempotent(t *testing.T) {
	app := newApplication(context.Background(), appConfig{DesktopAddr: "127.0.0.1:0", AdminToken: "token"}, appDependencies{
		resolveClaude: func(string) (string, error) { return "", errors.New("missing") },
	})
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeQuitClosesEngineAndCancelsContext(t *testing.T) {
	instance := &fakeManagedEngine{}
	app := newApplication(context.Background(), appConfig{DesktopAddr: "127.0.0.1:0", ClaudeBin: "claude", AdminToken: "token"}, appDependencies{
		resolveClaude: func(string) (string, error) { return "/tmp/claude", nil },
		detectVersion: func(string) (string, error) { return "1.2.3", nil },
		newEngine:     func(engine.Config) managedEngine { return instance },
	})
	if err := app.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	commands := newNativeCommands(app, cancel)
	commands.Quit()
	if !instance.closed.Load() {
		t.Fatal("native quit returned before the engine closed")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("native quit did not cancel the application context")
	}
}

func assertAppHTTPStatus(t *testing.T, handler http.Handler, path string, want int) {
	t.Helper()
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != want {
		t.Fatalf("%s status=%d want=%d body=%s", path, w.Code, want, w.Body.String())
	}
}

var _ managedEngine = (*fakeManagedEngine)(nil)
var _ = desktop.AppStatus{}
