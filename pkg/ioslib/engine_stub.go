//go:build !ios

package ioslib

import "errors"

var ErrIOSOnly = errors.New("iOS engine requires an iOS build")

func startPlatform(_, _, _, _ string, _ PacketFlow) (*Engine, error) {
	return nil, ErrIOSOnly
}
