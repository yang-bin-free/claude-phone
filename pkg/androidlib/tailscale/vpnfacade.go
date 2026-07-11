//go:build android

// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package tailscale

import (
	"sync"

	"tailscale.com/net/dns"
	"tailscale.com/wgengine/router"
)

var (
	_ router.Router      = (*VPNFacade)(nil)
	_ dns.OSConfigurator = (*VPNFacade)(nil)
)

// VPNFacade is an implementation of both wgengine.Router and
// dns.OSConfigurator. When ReconfigureVPN is called by the backend, SetBoth
// gets called.
type VPNFacade struct {
	SetBoth func(rcfg *router.Config, dcfg *dns.OSConfig) error

	// GetBaseConfigFunc optionally specifies a function to return the current DNS
	// config in response to GetBaseConfig.
	GetBaseConfigFunc func() (dns.OSConfig, error)

	// InitialMTU is the MTU the tun should be initialized with.
	InitialMTU uint32

	mu        sync.Mutex
	didSetMTU bool
	rcfg      *router.Config
	dcfg      *dns.OSConfig
}

// Up implements wgengine.router.
func (vf *VPNFacade) Up() error {
	return nil
}

// Set implements wgengine.router.
func (vf *VPNFacade) Set(rcfg *router.Config) error {
	vf.mu.Lock()
	defer vf.mu.Unlock()
	if vf.rcfg.Equal(rcfg) {
		return nil
	}
	if !vf.didSetMTU {
		vf.didSetMTU = true
		rcfg.NewMTU = int(vf.InitialMTU)
	}
	vf.rcfg = rcfg
	return nil
}

// UpdateMagicsockPort implements wgengine.Router.
func (vf *VPNFacade) UpdateMagicsockPort(_ uint16, _ string) error {
	return nil
}

// SetDNS implements dns.OSConfigurator.
func (vf *VPNFacade) SetDNS(dcfg dns.OSConfig) error {
	vf.mu.Lock()
	defer vf.mu.Unlock()
	if vf.dcfg != nil && vf.dcfg.Equal(dcfg) {
		return nil
	}
	vf.dcfg = &dcfg
	return nil
}

// SupportsSplitDNS implements dns.OSConfigurator.
func (vf *VPNFacade) SupportsSplitDNS() bool {
	return false
}

// GetBaseConfig implements dns.OSConfigurator.
func (vf *VPNFacade) GetBaseConfig() (dns.OSConfig, error) {
	if vf.GetBaseConfigFunc == nil {
		return dns.OSConfig{}, dns.ErrGetBaseConfigNotSupported
	}
	return vf.GetBaseConfigFunc()
}

// Close implements wgengine.router and dns.OSConfigurator.
func (vf *VPNFacade) Close() error {
	return vf.SetBoth(nil, nil)
}

// ReconfigureVPN is the method value passed to wgengine.Config.ReconfigureVPN.
func (vf *VPNFacade) ReconfigureVPN() error {
	vf.mu.Lock()
	defer vf.mu.Unlock()
	return vf.SetBoth(vf.rcfg, vf.dcfg)
}
