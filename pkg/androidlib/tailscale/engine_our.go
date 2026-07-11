//go:build android

// SPDX-License-Identifier: MIT

package tailscale

import (
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"sync"

	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnlocal"
	"tailscale.com/ipn/store"
	"tailscale.com/net/dns"
	"tailscale.com/net/netmon"
	"tailscale.com/net/netns"
	"tailscale.com/net/tsdial"
	"tailscale.com/tsd"
	"tailscale.com/types/logger"
	"tailscale.com/types/logid"
	"tailscale.com/wgengine"
	"tailscale.com/wgengine/netstack"
	"tailscale.com/wgengine/router"
)

// EngineBackend is a simplified version of Tailscale's backend,
// tailored for Claude Phone. It starts the Tailscale engine,
// manages the VPN tun fd, and registers the protect callback.
type EngineBackend struct {
	engine     wgengine.Engine
	localBE    *ipnlocal.LocalBackend
	sys        *tsd.System
	devices    *multiTUN
	netMon     *netmon.Monitor
	lastCfg    *router.Config
	lastDNSCfg *dns.OSConfig

	appCtx AppContext
	mu     sync.Mutex
}

// StartEngine initializes and starts the Tailscale networking engine.
// This is called from Kotlin after the VPN service is set up.
func StartEngine(dataDir string, appCtx AppContext) (*EngineBackend, error) {
	return StartEngineWithConfig(dataDir, "claude-phone", "", "", appCtx)
}

// StartEngineWithConfig starts the engine with persistent state and optional
// non-interactive tailnet enrollment settings.
func StartEngineWithConfig(dataDir, hostname, authKey, controlURL string, appCtx AppContext) (*EngineBackend, error) {
	b := &EngineBackend{
		devices: newTUNDevices(),
		appCtx:  appCtx,
	}

	sys := tsd.NewSystem()

	logf := logger.Logf(log.Printf)
	stateStore, err := store.New(logf, filepath.Join(dataDir, "tailscaled.state"))
	if err != nil {
		return nil, fmt.Errorf("open state store: %w", err)
	}
	sys.Set(stateStore)

	netMon, err := netmon.New(sys.Bus.Get(), logf)
	if err != nil {
		log.Printf("netmon.New: %v", err)
	}
	b.netMon = netMon

	// Register interface getter
	netmon.RegisterInterfaceGetter(b.getInterfaces)

	// Create dialer
	dialer := new(tsdial.Dialer)

	// Create VPNFacade as router + DNS configurator
	vf := &VPNFacade{
		SetBoth: b.setCfg,
	}

	// Create userspace engine with multiTUN and VPNFacade.
	engine, err := wgengine.NewUserspaceEngine(logf, wgengine.Config{
		Tun:            b.devices,
		Router:         vf,
		DNS:            vf,
		ReconfigureVPN: vf.ReconfigureVPN,
		Dialer:         dialer,
		SetSubsystem:   sys.Set,
		NetMon:         b.netMon,
	})
	if err != nil {
		return nil, fmt.Errorf("NewUserspaceEngine: %v", err)
	}
	sys.Set(engine)

	// Create netstack for userspace TCP/UDP
	ns, err := netstack.Create(logf, sys.Tun.Get(), engine, sys.MagicSock.Get(), dialer, sys.DNSManager.Get(), sys.ProxyMapper())
	if err != nil {
		return nil, fmt.Errorf("netstack.Create: %w", err)
	}
	sys.Set(ns)
	ns.ProcessLocalIPs = false
	ns.ProcessSubnets = true
	sys.NetstackRouter.Set(true)

	if w, ok := sys.Tun.GetOK(); ok {
		w.Start()
	}

	// Create LocalBackend (Tailscale control + networking)
	lb, err := ipnlocal.NewLocalBackend(logf, logid.PublicID{}, sys, 0)
	if err != nil {
		engine.Close()
		return nil, fmt.Errorf("NewLocalBackend: %v", err)
	}
	if err := ns.Start(lb); err != nil {
		return nil, fmt.Errorf("startNetstack: %w", err)
	}

	b.engine = engine
	b.localBE = lb
	b.sys = sys
	lb.SetVarRoot(dataDir)

	prefs := ipn.NewPrefs()
	prefs.Hostname = hostname
	prefs.WantRunning = true
	if controlURL != "" {
		prefs.ControlURL = controlURL
	}

	// Start the backend (this connects to Tailscale coordination server)
	go func() {
		if err := lb.Start(ipn.Options{UpdatePrefs: prefs, AuthKey: authKey}); err != nil {
			log.Printf("Failed to start LocalBackend: %s", err)
		}
	}()

	return b, nil
}

// RequestVPN is called from Kotlin when VPN service is ready.
// It registers the protect callback and updates VPN state.
func (b *EngineBackend) RequestVPN(service IPNService) {
	log.Printf("RequestVPN: registering protect callback")

	// Register protect callback at netns level (not per-socket).
	// This ensures ALL sockets created by the Tailscale engine are protected,
	// preventing routing loops through the VPN tunnel.
	netns.SetAndroidProtectFunc(func(fd int) error {
		if !service.Protect(int32(fd)) {
			log.Printf("[unexpected] VpnService.protect(%d) returned false", fd)
		}
		return nil
	})

	vpnState.service = service

	// Rebind magicsock to pick up the protect function
	b.localBE.DebugRebind()
}

// DisconnectVPN is called from Kotlin when VPN is disconnected.
func (b *EngineBackend) DisconnectVPN(service IPNService) {
	if vpnState.service != nil && vpnState.service.ID() == service.ID() {
		b.devices.Down()
		b.closeTUNs()
		netns.SetAndroidProtectFunc(nil)
		vpnState.service = nil
	}
}

func (b *EngineBackend) setCfg(rcfg *router.Config, dcfg *dns.OSConfig) error {
	if rcfg == nil {
		return nil
	}
	if b.isConfigNonNilAndDifferent(rcfg, dcfg) {
		return b.updateTUN(rcfg, dcfg)
	}
	return nil
}

func (b *EngineBackend) isConfigNonNilAndDifferent(rcfg *router.Config, dcfg *dns.OSConfig) bool {
	if reflect.DeepEqual(rcfg, b.lastCfg) && reflect.DeepEqual(dcfg, b.lastDNSCfg) {
		return false
	}
	return rcfg != nil
}

// GetLocalBackend returns the LocalBackend for WebSocket client to use
// (e.g., to check Tailscale network status, resolve addresses).
func (b *EngineBackend) GetLocalBackend() *ipnlocal.LocalBackend {
	return b.localBE
}

// Close shuts down the engine.
func (b *EngineBackend) Close() {
	if b.engine != nil {
		b.engine.Close()
	}
}
