// Package engine exposes the headless Claude Phone engine over WebSocket.
package engine

import "time"

const DefaultAddr = "127.0.0.1:9876"

type Config struct {
	Addr                 string
	AgentVersion         string
	ClaudeVersion        string
	ClaudeBin            string
	DefaultWorkingDir    string
	DefaultPermission    string
	MaxConcurrentSession int
	WriteTimeout         time.Duration
	DeviceTokens         map[string]string
}

func (c Config) withDefaults() Config {
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}
	if c.AgentVersion == "" {
		c.AgentVersion = "0.1.0-dev"
	}
	if c.ClaudeVersion == "" {
		c.ClaudeVersion = "unknown"
	}
	if c.ClaudeBin == "" {
		c.ClaudeBin = "claude"
	}
	if c.DefaultWorkingDir == "" {
		c.DefaultWorkingDir = "."
	}
	if c.DefaultPermission == "" {
		c.DefaultPermission = "default"
	}
	if c.MaxConcurrentSession <= 0 {
		c.MaxConcurrentSession = 5
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 5 * time.Second
	}
	return c
}
