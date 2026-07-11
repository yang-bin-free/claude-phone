//go:build !darwin

package engine

type noopPowerInhibitor struct{}

func newPowerInhibitor() powerInhibitor   { return noopPowerInhibitor{} }
func (noopPowerInhibitor) Acquire() error { return nil }
func (noopPowerInhibitor) Release() error { return nil }
func (noopPowerInhibitor) Active() bool   { return false }
