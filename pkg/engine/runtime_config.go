package engine

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type runtimeConfig struct {
	DefaultWorkingDir     string `yaml:"defaultWorkingDir"`
	DefaultPermission     string `yaml:"defaultPermission"`
	MaxConcurrentSessions int    `yaml:"maxConcurrentSessions"`
}

func (e *Engine) runtimeConfig() runtimeConfig {
	e.configMu.RLock()
	defer e.configMu.RUnlock()
	return e.runtime
}

func (e *Engine) reloadRuntimeConfig() error {
	b, err := os.ReadFile(filepath.Join(e.cfg.DataDir, "config.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var next runtimeConfig
	if err := yaml.Unmarshal(b, &next); err != nil {
		return err
	}
	current := e.runtimeConfig()
	if next.DefaultWorkingDir == "" {
		next.DefaultWorkingDir = current.DefaultWorkingDir
	}
	if next.DefaultPermission == "" {
		next.DefaultPermission = current.DefaultPermission
	}
	if next.MaxConcurrentSessions <= 0 {
		next.MaxConcurrentSessions = current.MaxConcurrentSessions
	}
	e.configMu.Lock()
	e.runtime = next
	e.configMu.Unlock()
	e.manager.SetMaxConcurrent(next.MaxConcurrentSessions)
	return nil
}

func (e *Engine) watchRuntimeConfig() {
	interval := e.cfg.ConfigPollInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = e.reloadRuntimeConfig()
		case <-e.stopWatch:
			return
		}
	}
}
