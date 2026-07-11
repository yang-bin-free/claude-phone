//go:build ios

package ioslib

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/tailscale/wireguard-go/tun"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnlocal"
	"tailscale.com/ipn/store"
	"tailscale.com/net/dns"
	"tailscale.com/net/netmon"
	"tailscale.com/net/tsdial"
	"tailscale.com/tsd"
	"tailscale.com/types/logger"
	"tailscale.com/types/logid"
	"tailscale.com/wgengine"
	"tailscale.com/wgengine/router"
)

type packetBatch struct {
	Packets   []string `json:"packets"`
	Protocols []int32  `json:"protocols"`
}

type networkSettings struct {
	MTU       int      `json:"mtu"`
	Addresses []string `json:"addresses"`
	Routes    []string `json:"routes"`
	DNS       []string `json:"dns"`
}

type flowTUN struct {
	flow   PacketFlow
	events chan tun.Event
	done   chan struct{}
	once   sync.Once
}

func newFlowTUN(flow PacketFlow) *flowTUN {
	t := &flowTUN{flow: flow, events: make(chan tun.Event, 1), done: make(chan struct{})}
	t.events <- tun.EventUp
	return t
}
func (*flowTUN) File() *os.File             { return nil }
func (*flowTUN) MTU() (int, error)          { return 1280, nil }
func (*flowTUN) Name() (string, error)      { return "ClaudePhone", nil }
func (t *flowTUN) Events() <-chan tun.Event { return t.events }
func (*flowTUN) BatchSize() int             { return 16 }
func (t *flowTUN) Close() error             { t.once.Do(func() { close(t.done); close(t.events) }); return nil }
func (t *flowTUN) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	select {
	case <-t.done:
		return 0, os.ErrClosed
	default:
	}
	raw, err := t.flow.ReadPackets()
	if err != nil {
		return 0, err
	}
	var batch packetBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return 0, err
	}
	n := 0
	for i, encoded := range batch.Packets {
		if i >= len(bufs) || i >= len(sizes) {
			break
		}
		packet, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		if len(packet)+offset > len(bufs[i]) {
			continue
		}
		copy(bufs[i][offset:], packet)
		sizes[i] = len(packet)
		n++
	}
	if n == 0 {
		return 0, io.ErrNoProgress
	}
	return n, nil
}
func (t *flowTUN) Write(bufs [][]byte, offset int) (int, error) {
	batch := packetBatch{Packets: make([]string, 0, len(bufs)), Protocols: make([]int32, 0, len(bufs))}
	for _, buffer := range bufs {
		if offset > len(buffer) {
			continue
		}
		packet := buffer[offset:]
		batch.Packets = append(batch.Packets, base64.StdEncoding.EncodeToString(packet))
		protocol := int32(0)
		if len(packet) > 0 {
			if packet[0]>>4 == 4 {
				protocol = 2
			} else if packet[0]>>4 == 6 {
				protocol = 30
			}
		}
		batch.Protocols = append(batch.Protocols, protocol)
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		return 0, err
	}
	if err := t.flow.WritePackets(raw); err != nil {
		return 0, err
	}
	return len(batch.Packets), nil
}

func startPlatform(dataDir, hostname, authKey, controlURL string, flow PacketFlow) (*Engine, error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}
	logf := logger.Logf(func(format string, args ...any) {
		line := logger.Logf(log.Printf)
		line(format, args...)
		flow.Log("tailscale: " + format)
	})
	sys := tsd.NewSystem()
	stateStore, err := store.New(logf, filepath.Join(dataDir, "tailscaled.state"))
	if err != nil {
		return nil, err
	}
	sys.Set(stateStore)
	netMon, err := netmon.New(sys.Bus.Get(), logf)
	if err != nil {
		return nil, err
	}
	dialer := new(tsdial.Dialer)
	device := newFlowTUN(flow)
	callback := &router.CallbackRouter{InitialMTU: 1280, SetBoth: func(rcfg *router.Config, dcfg *dns.OSConfig) error {
		settings := networkSettings{MTU: 1280}
		if rcfg != nil {
			for _, addr := range rcfg.LocalAddrs {
				settings.Addresses = append(settings.Addresses, addr.String())
			}
			for _, route := range rcfg.Routes {
				settings.Routes = append(settings.Routes, route.Masked().String())
			}
		}
		if dcfg != nil {
			for _, server := range dcfg.Nameservers {
				settings.DNS = append(settings.DNS, server.String())
			}
		}
		raw, _ := json.Marshal(settings)
		return flow.Configure(string(raw))
	}}
	wg, err := wgengine.NewUserspaceEngine(logf, wgengine.Config{Tun: device, Router: callback, DNS: callback, Dialer: dialer, SetSubsystem: sys.Set, NetMon: netMon})
	if err != nil {
		device.Close()
		return nil, err
	}
	sys.Set(wg)
	lb, err := ipnlocal.NewLocalBackend(logf, logid.PublicID{}, sys, 0)
	if err != nil {
		wg.Close()
		return nil, err
	}
	lb.SetVarRoot(dataDir)
	prefs := ipn.NewPrefs()
	prefs.Hostname = hostname
	prefs.WantRunning = true
	if controlURL != "" {
		prefs.ControlURL = controlURL
	}
	if err := lb.Start(ipn.Options{UpdatePrefs: prefs, AuthKey: authKey}); err != nil {
		wg.Close()
		return nil, err
	}
	return newEngine("running", func() error { lb.Shutdown(); wg.Close(); netMon.Close(); return device.Close() }), nil
}
