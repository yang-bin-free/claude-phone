// Package ioslib exposes the narrow gomobile API used by the iOS Packet Tunnel.
package ioslib

import (
	"errors"
	"sync"
)

var ErrPacketFlowRequired = errors.New("packet flow is required")

type PacketFlow interface {
	Configure(networkSettingsJSON string) error
	ReadPackets() ([]byte, error)
	WritePackets(packetBatchJSON []byte) error
	Log(line string)
}

type Engine struct {
	mu     sync.RWMutex
	stop   func() error
	status string
}

func Start(dataDir, hostname, authKey, controlURL string, flow PacketFlow) (*Engine, error) {
	if flow == nil {
		return nil, ErrPacketFlowRequired
	}
	if hostname == "" {
		hostname = "claude-phone-ios"
	}
	return startPlatform(dataDir, hostname, authKey, controlURL, flow)
}

func (e *Engine) Stop() error {
	e.mu.Lock()
	stop := e.stop
	e.stop = nil
	e.status = "stopped"
	e.mu.Unlock()
	if stop != nil {
		return stop()
	}
	return nil
}

func (e *Engine) Status() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

func newEngine(status string, stop func() error) *Engine {
	return &Engine{status: status, stop: stop}
}
