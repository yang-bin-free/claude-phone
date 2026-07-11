//go:build android

// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause

package tailscale

import (
	"log"
	"os"
	"runtime/debug"
	"sync"

	"github.com/tailscale/wireguard-go/tun"
	"tailscale.com/syncs"
)

// multiTUN implements a tun.Device that supports multiple
// underlying devices. This is necessary because Android VPN devices
// have static configurations and wgengine.NewUserspaceEngine
// assumes a single static tun.Device.
type multiTUN struct {
	devices  chan tun.Device
	events   chan tun.Event
	close    chan struct{}
	closeErr chan error

	reads        chan ioRequest
	writes       chan ioRequest
	mtus         chan chan mtuReply
	names        chan chan nameReply
	shutdowns    chan struct{}
	shutdownDone chan struct{}

	downMu sync.Mutex
	downCh syncs.AtomicValue[chan struct{}]
	down   bool
}

type tunDevice struct {
	dev       tun.Device
	close     chan struct{}
	closeDone chan error
	readDone  chan struct{}
}

type ioRequest struct {
	data   [][]byte
	sizes  []int
	offset int
	reply  chan<- ioReply
}

type ioReply struct {
	count int
	err   error
}

type mtuReply struct {
	mtu int
	err error
}

type nameReply struct {
	name string
	err  error
}

const defaultMTU = 1280

func newTUNDevices() *multiTUN {
	d := &multiTUN{
		devices:      make(chan tun.Device),
		events:       make(chan tun.Event),
		close:        make(chan struct{}),
		closeErr:     make(chan error),
		reads:        make(chan ioRequest),
		writes:       make(chan ioRequest),
		mtus:         make(chan chan mtuReply),
		names:        make(chan chan nameReply),
		shutdowns:    make(chan struct{}),
		shutdownDone: make(chan struct{}),
		down:         true,
	}
	downCh := make(chan struct{})
	d.downCh.Store(downCh)
	close(downCh)
	go d.run()
	return d
}

func (d *multiTUN) run() {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("panic in multiTUN.run %s: %s", p, debug.Stack())
			panic(p)
		}
	}()

	var devices []*tunDevice
	var readDone chan struct{}
	var runDone chan error
	for {
		select {
		case <-readDone:
			n := copy(devices, devices[1:])
			devices = devices[:n]
			if len(devices) > 0 {
				dev := devices[0]
				readDone = dev.readDone
				go d.readFrom(dev)
			}
		case <-runDone:
			if len(devices) > 0 {
				dev := devices[len(devices)-1]
				runDone = dev.closeDone
				go d.runDevice(dev)
			}
		case <-d.shutdowns:
			for _, dev := range devices {
				close(dev.close)
				<-dev.closeDone
				<-dev.readDone
			}
			devices = nil
			d.shutdownDone <- struct{}{}
		case <-d.close:
			var derr error
			for range devices {
				if err := <-devices[0].closeDone; err != nil {
					derr = err
				}
			}
			d.closeErr <- derr
			return
		case dev := <-d.devices:
			if len(devices) > 0 {
				prev := devices[len(devices)-1]
				close(prev.close)
			}
			wrap := &tunDevice{
				dev:       dev,
				close:     make(chan struct{}),
				closeDone: make(chan error),
				readDone:  make(chan struct{}, 1),
			}
			if len(devices) == 0 {
				readDone = wrap.readDone
				go d.readFrom(wrap)
				runDone = wrap.closeDone
				go d.runDevice(wrap)
			}
			devices = append(devices, wrap)
		case m := <-d.mtus:
			r := mtuReply{mtu: defaultMTU}
			if len(devices) > 0 {
				dev := devices[len(devices)-1]
				r.mtu, r.err = dev.dev.MTU()
			}
			m <- r
		case n := <-d.names:
			var r nameReply
			if len(devices) > 0 {
				dev := devices[len(devices)-1]
				r.name, r.err = dev.dev.Name()
			}
			n <- r
		}
	}
}

func (d *multiTUN) readFrom(dev *tunDevice) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("panic in multiTUN.readFrom %s: %s", p, debug.Stack())
			panic(p)
		}
	}()
	defer func() { dev.readDone <- struct{}{} }()
	for {
		select {
		case r := <-d.reads:
			n, err := dev.dev.Read(r.data, r.sizes, r.offset)
			stop := false
			if err != nil {
				select {
				case <-dev.close:
					stop = true
					err = nil
				default:
				}
			}
			r.reply <- ioReply{n, err}
			if stop {
				return
			}
		case <-d.close:
			return
		}
	}
}

func (d *multiTUN) runDevice(dev *tunDevice) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("panic in multiTUN.runDevice %s: %s", p, debug.Stack())
			panic(p)
		}
	}()
	defer func() { dev.closeDone <- dev.dev.Close() }()
	go func() {
		defer func() {
			if p := recover(); p != nil {
				log.Printf("panic in multiTUN.runDevice.events %s: %s", p, debug.Stack())
				panic(p)
			}
		}()
		for {
			select {
			case e := <-dev.dev.Events():
				d.events <- e
			case <-dev.close:
				return
			}
		}
	}()
	for {
		select {
		case w := <-d.writes:
			n, err := dev.dev.Write(w.data, w.offset)
			w.reply <- ioReply{n, err}
		case <-dev.close:
			return
		case <-d.close:
			return
		}
	}
}

func (d *multiTUN) add(dev tun.Device) {
	d.devices <- dev
}

func (d *multiTUN) Up() bool {
	d.downMu.Lock()
	defer d.downMu.Unlock()
	if !d.down {
		return false
	}
	d.downCh.Store(make(chan struct{}))
	d.down = false
	return true
}

func (d *multiTUN) Down() bool {
	d.downMu.Lock()
	defer d.downMu.Unlock()
	if d.down {
		return false
	}
	close(d.downCh.Load())
	d.down = true
	return true
}

func (d *multiTUN) File() *os.File {
	panic("not available on Android")
}

func (d *multiTUN) Read(data [][]byte, sizes []int, offset int) (int, error) {
	r := make(chan ioReply)
	select {
	case d.reads <- ioRequest{data, sizes, offset, r}:
		rep := <-r
		return rep.count, rep.err
	case <-d.close:
		return 0, os.ErrClosed
	}
}

func (d *multiTUN) Write(data [][]byte, offset int) (int, error) {
	r := make(chan ioReply)
	select {
	case d.writes <- ioRequest{data, nil, offset, r}:
		rep := <-r
		return rep.count, rep.err
	case <-d.downCh.Load():
		return 0, nil
	case <-d.close:
		return 0, os.ErrClosed
	}
}

func (d *multiTUN) MTU() (int, error) {
	r := make(chan mtuReply)
	d.mtus <- r
	rep := <-r
	return rep.mtu, rep.err
}

func (d *multiTUN) Name() (string, error) {
	r := make(chan nameReply)
	d.names <- r
	rep := <-r
	return rep.name, rep.err
}

func (d *multiTUN) Events() <-chan tun.Event {
	return d.events
}

func (d *multiTUN) Shutdown() {
	d.shutdowns <- struct{}{}
	<-d.shutdownDone
}

func (d *multiTUN) Close() error {
	close(d.close)
	return <-d.closeErr
}

func (d *multiTUN) BatchSize() int {
	return 1
}
