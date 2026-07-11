//go:build darwin

package engine

import (
	"os/exec"
	"sync"
)

type caffeinateInhibitor struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

func newPowerInhibitor() powerInhibitor { return &caffeinateInhibitor{} }

func (p *caffeinateInhibitor) Acquire() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		return nil
	}
	cmd := exec.Command("/usr/bin/caffeinate", "-s")
	if err := cmd.Start(); err != nil {
		return err
	}
	p.cmd = cmd
	return nil
}

func (p *caffeinateInhibitor) Release() error {
	p.mu.Lock()
	cmd := p.cmd
	p.cmd = nil
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	_ = cmd.Process.Kill()
	return cmd.Wait()
}

func (p *caffeinateInhibitor) Active() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}
