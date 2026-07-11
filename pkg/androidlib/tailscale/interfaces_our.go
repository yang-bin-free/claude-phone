//go:build android

// SPDX-License-Identifier: MIT

package tailscale

import (
	"log"
	"strings"

	"tailscale.com/net/netmon"
)

// getInterfaces reports network interfaces from the Android device.
func (b *EngineBackend) getInterfaces() ([]netmon.Interface, error) {
	if b.appCtx == nil {
		return nil, nil
	}
	jsonStr, err := b.appCtx.GetInterfacesAsJson()
	if err != nil {
		return nil, err
	}
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil, nil
	}

	// For P0b we use a simple approach; full ifaceparse will be added later.
	log.Printf("getInterfaces: received JSON (%d bytes)", len(jsonStr))
	return nil, nil
}
