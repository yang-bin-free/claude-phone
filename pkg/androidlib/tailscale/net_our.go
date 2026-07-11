//go:build android

// SPDX-License-Identifier: MIT

package tailscale

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"syscall"

	"github.com/tailscale/wireguard-go/tun"
	"tailscale.com/net/dns"
	"tailscale.com/net/netmon"
	"tailscale.com/wgengine/router"
)

var errVPNNotPrepared = errors.New("VPN service not prepared or was revoked")

// VPNServiceState holds the current VPN service reference and tun fd.
type VPNServiceState struct {
	service  IPNService
	fd       int32
	detached bool
}

var vpnState = &VPNServiceState{}

// updateTUN creates a new tun device from the VPN service builder.
// This is the core function that receives the tun fd from Kotlin VpnService
// and passes it to the Tailscale engine.
func (b *EngineBackend) updateTUN(rcfg *router.Config, dcfg *dns.OSConfig) error {
	if rcfg == nil || len(rcfg.LocalAddrs) == 0 {
		b.devices.Down()
		b.closeTUNs()
		return nil
	}

	builder := vpnState.service.NewBuilder()

	if err := builder.SetMTU(defaultMTU); err != nil {
		return err
	}

	if dcfg != nil {
		for _, ns := range dcfg.Nameservers {
			if err := builder.AddDNSServer(ns.String()); err != nil {
				return err
			}
		}
		for _, dom := range dcfg.SearchDomains {
			if err := builder.AddSearchDomain(dom.WithoutTrailingDot()); err != nil {
				return err
			}
		}
	}

	// Add routes
	for _, route := range rcfg.Routes {
		route = route.Masked()
		if err := builder.AddRoute(route.Addr().String(), int32(route.Bits())); err != nil {
			return err
		}
	}

	// Add addresses
	for _, addr := range rcfg.LocalAddrs {
		if err := builder.AddAddress(addr.Addr().String(), int32(addr.Bits())); err != nil {
			return err
		}
	}

	// Establish the VPN tunnel; this returns a ParcelFileDescriptor.
	parcelFD, err := builder.Establish()
	if err != nil {
		if strings.Contains(err.Error(), "INTERACT_ACROSS_USERS") {
			vpnState.service.UpdateVpnStatus(false)
			return errors.New("VPN cannot be created due to Android multi-user bug")
		}
		return fmt.Errorf("VpnService.Builder.establish: %v", err)
	}

	vpnState.service.UpdateVpnStatus(true)

	if parcelFD == nil {
		return errVPNNotPrepared
	}

	// Detach the fd from the ParcelFileDescriptor
	tunFD, err := parcelFD.Detach()
	vpnState.fd = tunFD
	vpnState.detached = true

	if err != nil {
		return fmt.Errorf("detachFd: %v", err)
	}

	// Create a wireguard-go TUN device from the file descriptor.
	tunDev, _, err := tun.CreateUnmonitoredTUNFromFD(int(tunFD))
	if err != nil {
		closeFd()
		return err
	}

	b.devices.add(tunDev)

	if b.devices.Up() {
		log.Printf("tunnel brought up")
	}

	b.lastCfg = rcfg
	b.lastDNSCfg = dcfg
	return nil
}

func closeFd() error {
	if vpnState.fd != -1 && vpnState.detached {
		err := syscall.Close(int(vpnState.fd))
		vpnState.fd = -1
		vpnState.detached = false
		return fmt.Errorf("error closing file descriptor: %w", err)
	}
	return nil
}

func (b *EngineBackend) closeTUNs() {
	b.lastCfg = nil
	b.lastDNSCfg = nil
	b.devices.Shutdown()
}

// NetworkChanged is called when the Android network changes.
func (b *EngineBackend) NetworkChanged(ifname string) {
	netmon.UpdateLastKnownDefaultRouteInterface(ifname)
	if b.netMon != nil {
		b.netMon.InjectEvent()
	}
}
