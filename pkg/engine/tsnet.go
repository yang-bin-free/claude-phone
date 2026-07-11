package engine

import (
	"net"

	"tailscale.com/tsnet"
)

type TSNetConfig struct {
	Hostname   string
	Dir        string
	AuthKey    string
	ControlURL string
}

func (e *Engine) ServeTSNet(cfg TSNetConfig, addr string) error {
	s := &tsnet.Server{
		Hostname:   cfg.Hostname,
		Dir:        cfg.Dir,
		AuthKey:    cfg.AuthKey,
		ControlURL: cfg.ControlURL,
	}
	defer s.Close()
	ln, err := s.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return e.Serve(ln)
}

func (e *Engine) ServeListener(ln net.Listener) error {
	return e.Serve(ln)
}
