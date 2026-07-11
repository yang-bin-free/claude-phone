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
	for id := range e.clients {
		connected = append(connected, id)
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
		snaps = append(snaps, SessionSnapshot{
			SessionID:   s.ID,
			Name:        s.Name,
			Status:      s.Status,
			Owner:       s.Owner,
			Subscribers: s.Subscribers(),
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
