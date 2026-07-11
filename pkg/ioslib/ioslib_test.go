//go:build !ios

package ioslib

import (
	"errors"
	"testing"
)

func TestEngineRejectsMissingPacketFlow(t *testing.T) {
	if _, err := Start(t.TempDir(), "claude-phone-ios", "", "", nil); !errors.Is(err, ErrPacketFlowRequired) {
		t.Fatalf("error = %v, want ErrPacketFlowRequired", err)
	}
}

func TestHostBuildReportsIOSOnly(t *testing.T) {
	_, err := Start(t.TempDir(), "claude-phone-ios", "", "", stubFlow{})
	if !errors.Is(err, ErrIOSOnly) {
		t.Fatalf("error = %v, want ErrIOSOnly", err)
	}
}

type stubFlow struct{}

func (stubFlow) Configure(string) error       { return nil }
func (stubFlow) ReadPackets() ([]byte, error) { return nil, nil }
func (stubFlow) WritePackets([]byte) error    { return nil }
func (stubFlow) Log(string)                   {}
