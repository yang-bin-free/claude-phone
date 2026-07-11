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

func TestManager_ListIsSortedByCreatedAt(t *testing.T) {
	n := int64(0)
	m := NewManager(ManagerConfig{
		MaxConcurrent: 5,
		IDFunc: func() string {
			n++
			return "sess-" + string(rune('0'+n))
		},
		Now: func() int64 {
			n++
			return n
		},
	})

	s1, _ := m.Create("a", "/p", "default", "d")
	s2, _ := m.Create("b", "/p", "default", "d")
	s3, _ := m.Create("c", "/p", "default", "d")

	got := m.List()
	if len(got) != 3 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID != s1.ID || got[1].ID != s2.ID || got[2].ID != s3.ID {
		t.Fatalf("list order wrong: %s %s %s", got[0].ID, got[1].ID, got[2].ID)
	}
}
