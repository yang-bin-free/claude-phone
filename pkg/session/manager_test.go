package session

import (
	"errors"
	"testing"
)

func newTestManager(limit int) *Manager {
	n := 0
	return NewManager(ManagerConfig{
		MaxConcurrent: limit,
		IDFunc: func() string {
			n++
			return "sess-" + string(rune('0'+n))
		},
	})
}

func TestManager_CreateAndGet(t *testing.T) {
	m := newTestManager(5)
	s, err := m.Create("车险", "/p", "bypassPermissions", "device-A")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, ok := m.Get(s.ID)
	if !ok || got.Name != "车险" {
		t.Fatalf("get failed: %v %v", got, ok)
	}
	if len(m.List()) != 1 {
		t.Fatalf("list len = %d", len(m.List()))
	}
}

func TestManager_ConcurrencyLimit(t *testing.T) {
	m := newTestManager(2)
	_, _ = m.Create("a", "/p", "default", "d")
	_, _ = m.Create("b", "/p", "default", "d")
	_, err := m.Create("c", "/p", "default", "d")
	if !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("want ErrSessionLimit, got %v", err)
	}
}
