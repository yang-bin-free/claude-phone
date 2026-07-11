package engine

import (
	"sort"
	"time"
)

// StatusReport describes the current runtime state of the agent.
type StatusReport struct {
	Addr                 string            `json:"addr"`
	AgentVersion         string            `json:"agentVersion"`
	ClaudeVersion        string            `json:"claudeVersion"`
	ClaudeBin            string            `json:"claudeBin"`
	DefaultWorkingDir    string            `json:"defaultWorkingDir"`
	DefaultPermission    string            `json:"defaultPermission"`
	MaxConcurrentSession int               `json:"maxConcurrentSession"`
	ConnectedDevices     []string          `json:"connectedDevices"`
	Sessions             []SessionSnapshot `json:"sessions"`
}

// SessionSnapshot captures a session plus whether a claude process is running.
type SessionSnapshot struct {
	SessionID   string   `json:"sessionId"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Owner       string   `json:"owner"`
	Subscribers []string `json:"subscribers"`
	CreatedAt   int64    `json:"createdAt"`
	Running     bool     `json:"running"`
	Health      string   `json:"health"`
	IdleSeconds int64    `json:"idleSeconds"`
}

// Status returns a snapshot of the current engine state.
func (e *Engine) Status() StatusReport {
	runtime := e.runtimeConfig()
	e.mu.RLock()
	connected := make([]string, 0, len(e.clients))
	for token := range e.clients {
		connected = append(connected, e.deviceDisplayName(token))
	}
	procs := make(map[string]struct{}, len(e.procs))
	for id := range e.procs {
		procs[id] = struct{}{}
	}
	activity := make(map[string]time.Time, len(e.activity))
	for id, last := range e.activity {
		activity[id] = last
	}
	e.mu.RUnlock()

	sort.Strings(connected)

	sessions := e.manager.List()
	snaps := make([]SessionSnapshot, 0, len(sessions))
	for _, s := range sessions {
		_, running := procs[s.ID]
		health, idle := "idle", int64(0)
		if running {
			health, idle = e.healthAt(activity[s.ID], time.Now())
		}
		subscribers := s.Subscribers()
		for i, token := range subscribers {
			subscribers[i] = e.safeDeviceDisplayName(token)
		}
		snaps = append(snaps, SessionSnapshot{
			SessionID:   s.ID,
			Name:        s.Name,
			Status:      s.Status,
			Owner:       e.safeDeviceDisplayName(s.Owner),
			Subscribers: subscribers,
			CreatedAt:   s.CreatedAt,
			Running:     running,
			Health:      health,
			IdleSeconds: idle,
		})
	}

	return StatusReport{
		Addr:                 e.cfg.Addr,
		AgentVersion:         e.cfg.AgentVersion,
		ClaudeVersion:        e.cfg.ClaudeVersion,
		ClaudeBin:            e.cfg.ClaudeBin,
		DefaultWorkingDir:    runtime.DefaultWorkingDir,
		DefaultPermission:    runtime.DefaultPermission,
		MaxConcurrentSession: runtime.MaxConcurrentSessions,
		ConnectedDevices:     connected,
		Sessions:             snaps,
	}
}

func (e *Engine) deviceDisplayName(token string) string {
	if name := e.cfg.DeviceTokens[token]; name != "" {
		return name
	}
	return "device-" + deviceTokenID(token)[:8]
}

func (e *Engine) safeDeviceDisplayName(token string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.deviceDisplayName(token)
}
