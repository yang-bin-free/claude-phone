package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/desktop"
)

func TestGenerateAdminToken(t *testing.T) {
	token, err := generateAdminToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(token) != 64 {
		t.Fatalf("token length=%d", len(token))
	}
}

func TestValidateDesktopAddr(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:0", "localhost:9877", "[::1]:9877"} {
		if err := validateDesktopAddr(addr); err != nil {
			t.Errorf("%s: %v", addr, err)
		}
	}
	for _, addr := range []string{":9877", "0.0.0.0:9877", "192.168.1.2:9877"} {
		if err := validateDesktopAddr(addr); err == nil {
			t.Errorf("%s: expected rejection", addr)
		}
	}
}

func TestDesktopServerStopsOnCancellation(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- desktop.Serve(ctx, ln, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not stop")
	}
}
