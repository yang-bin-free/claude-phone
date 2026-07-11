package engine

import (
	"encoding/json"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func (e *Engine) recordActivity(sessionID string) {
	e.mu.Lock()
	e.activity[sessionID] = time.Now()
	e.mu.Unlock()
}

func (e *Engine) healthAt(last, now time.Time) (string, int64) {
	if last.IsZero() {
		return "unknown", 0
	}
	idle := now.Sub(last)
	state := "healthy"
	if idle >= e.cfg.UnresponsiveAfter {
		state = "unresponsive"
	} else if idle >= e.cfg.StalledAfter {
		state = "stalled"
	}
	return state, int64(idle.Seconds())
}

func (e *Engine) monitorHealth() {
	ticker := time.NewTicker(e.cfg.HealthPollInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			for _, sess := range e.manager.List() {
				e.mu.Lock()
				last, running := e.activity[sess.ID]
				state, idle := e.healthAt(last, now)
				previous := e.healthState[sess.ID]
				if running {
					e.healthState[sess.ID] = state
				}
				e.mu.Unlock()
				if running && state != previous {
					payload, _ := json.Marshal(protocol.HealthMsg{Type: protocol.TypeHealth, SessionID: sess.ID, State: state, IdleSeconds: idle})
					sess.Broadcast(payload)
				}
			}
		case <-e.stopWatch:
			return
		}
	}
}
