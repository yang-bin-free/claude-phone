package engine

import (
	"testing"
	"time"
)

func TestHealthThresholds(t *testing.T) {
	e := New(Config{DataDir: t.TempDir(), StalledAfter: 2 * time.Minute, UnresponsiveAfter: 5 * time.Minute})
	defer e.Close()
	now := time.Unix(1000, 0)
	tests := []struct {
		idle time.Duration
		want string
	}{
		{time.Minute, "healthy"},
		{2 * time.Minute, "stalled"},
		{5 * time.Minute, "unresponsive"},
	}
	for _, tt := range tests {
		got, _ := e.healthAt(now.Add(-tt.idle), now)
		if got != tt.want {
			t.Fatalf("idle %s: got %s want %s", tt.idle, got, tt.want)
		}
	}
}
