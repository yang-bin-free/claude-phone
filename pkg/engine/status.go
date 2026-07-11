package engine

import "sort"

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
}

// Status returns a snapshot of the current engine state.
func (e *Engine) Status() StatusReport {
	e.mu.RLock()
	connected := make([]string, 0, len(e.clients))
	for token := range e.clients {
		connected = append(connected, e.deviceDisplayName(token))
	}
	procs := make(map[string]struct{}, len(e.procs))
	for id := range e.procs {
		procs[id] = struct{}{}
	}
	e.mu.RUnlock()

	sort.Strings(connected)

	sessions := e.manager.List()
	snaps := make([]SessionSnapshot, 0, len(sessions))
	for _, s := range sessions {
		_, running := procs[s.ID]
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
		})
	}

	return StatusReport{
		Addr:                 e.cfg.Addr,
		AgentVersion:         e.cfg.AgentVersion,
		ClaudeVersion:        e.cfg.ClaudeVersion,
		ClaudeBin:            e.cfg.ClaudeBin,
		DefaultWorkingDir:    e.cfg.DefaultWorkingDir,
		DefaultPermission:    e.cfg.DefaultPermission,
		MaxConcurrentSession: e.cfg.MaxConcurrentSession,
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
