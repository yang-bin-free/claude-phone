package session

import (
	"sync"
	"testing"
)

func TestSubscribeUnsubscribe(t *testing.T) {
	s := NewSession("sess1", "车险联调", "/p", "device-A")
	if s.Owner != "device-A" {
		t.Fatalf("owner=%q", s.Owner)
	}
	s.Subscribe("device-B")
	subs := s.Subscribers()
	if len(subs) != 2 {
		t.Fatalf("want 2 subs, got %v", subs)
	}
	s.Unsubscribe("device-B")
	if len(s.Subscribers()) != 1 {
		t.Fatalf("unsubscribe failed: %v", s.Subscribers())
	}
}

func TestBroadcastFanOut(t *testing.T) {
	s := NewSession("sess1", "n", "/p", "device-A")
	s.Subscribe("device-B")

	var mu sync.Mutex
	got := map[string]int{}
	s.SetSender(func(deviceID string, payload []byte) {
		mu.Lock()
		got[deviceID]++
		mu.Unlock()
	})
	s.Broadcast([]byte(`{"type":"token","content":"hi"}`))

	if got["device-A"] != 1 || got["device-B"] != 1 {
		t.Fatalf("fan-out wrong: %v", got)
	}
}
