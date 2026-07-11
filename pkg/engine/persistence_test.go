package engine

import (
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/session"
)

func TestEngineReloadsPersistedSessionAsDormant(t *testing.T) {
	dataDir := t.TempDir()
	e := New(Config{DataDir: dataDir})
	e.manager = session.NewManager(session.ManagerConfig{IDFunc: func() string { return "sess-persist" }, Now: func() int64 { return 123 }})
	s, err := e.manager.Create("Demo", "/tmp", "default", "device")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.history.CreateSession(s); err != nil {
		t.Fatal(err)
	}
	if err := e.history.Append(s.ID, []byte(`{"type":"text","content":"hello"}`)); err != nil {
		t.Fatal(err)
	}

	restarted := New(Config{DataDir: dataDir})
	loaded, ok := restarted.manager.Get(s.ID)
	if !ok || loaded.Status != "dormant" || loaded.Name != "Demo" || loaded.Permission != "default" {
		t.Fatalf("loaded=%+v ok=%v", loaded, ok)
	}
	if len(loaded.Subscribers()) != 0 {
		t.Fatalf("restored subscribers = %v, want none", loaded.Subscribers())
	}
	messages, err := restarted.history.Load(s.ID, 50)
	if err != nil || len(messages) != 1 {
		t.Fatalf("messages=%d err=%v", len(messages), err)
	}
}

func TestDormantRestoredSessionsDoNotConsumeActiveLimit(t *testing.T) {
	dataDir := t.TempDir()
	e := New(Config{DataDir: dataDir, MaxConcurrentSession: 1})
	old, err := e.manager.Create("old", "/tmp", "default", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.history.CreateSession(old); err != nil {
		t.Fatal(err)
	}

	restarted := New(Config{DataDir: dataDir, MaxConcurrentSession: 1})
	if _, err := restarted.manager.Create("new", "/tmp", "default", "owner"); err != nil {
		t.Fatalf("create with dormant history: %v", err)
	}
}
