package engine

import (
	"net"

	"tailscale.com/tsnet"
)

func (e *Engine) ServeTSNet(hostname, dir, addr string) error {
	s := &tsnet.Server{Hostname: hostname, Dir: dir}
	ln, err := s.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return e.Serve(ln)
}

func (e *Engine) ServeListener(ln net.Listener) error {
	return e.Serve(ln)
}
