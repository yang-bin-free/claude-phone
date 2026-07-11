// Package adminproto defines the loopback-only desktop administration protocol.
package adminproto

type Snapshot struct {
	Agent    AgentStatus      `json:"agent"`
	Devices  []DeviceSnapshot `json:"devices"`
	Projects []Project        `json:"projects"`
}

type Project struct {
	ProjectID  string `json:"projectId" yaml:"-"`
	Name       string `json:"name" yaml:"name"`
	Path       string `json:"path" yaml:"path"`
	Permission string `json:"permission" yaml:"permission,omitempty"`
}

type DeviceSnapshot struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Online   bool   `json:"online"`
}

type AgentStatus struct {
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

type SessionSnapshot struct {
	SessionID   string   `json:"sessionId"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Owner       string   `json:"owner"`
	Subscribers []string `json:"subscribers"`
	CreatedAt   int64    `json:"createdAt"`
	Running     bool     `json:"running"`
}

type StopSessionRequest struct {
	SessionID string `json:"sessionId"`
}
