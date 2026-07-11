package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/yang-bin-free/claude-phone/pkg/desktop"
	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

func main() {
	fs := flag.NewFlagSet("claude-phone", flag.ExitOnError)
	desktopAddr := fs.String("desktop-addr", "127.0.0.1:9877", "loopback desktop listen address")
	claudeBin := fs.String("claude-bin", "claude", "Claude CLI binary")
	workdir := fs.String("workdir", ".", "default Claude working directory")
	permission := fs.String("permission", "default", "default Claude permission mode")
	_ = fs.Parse(os.Args[1:])

	if err := validateDesktopAddr(*desktopAddr); err != nil {
		log.Fatal(err)
	}
	token, err := generateAdminToken()
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", *desktopAddr)
	if err != nil {
		log.Fatal(err)
	}

	e := engine.New(engine.Config{
		Addr: *desktopAddr, ClaudeBin: *claudeBin, DefaultWorkingDir: *workdir,
		DefaultPermission: *permission,
	})
	handler := desktop.NewHandler(e.Handler(), e.AdminHandler(token))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer e.Close()

	log.Printf("Claude Phone desktop service listening on http://%s", listener.Addr())
	if err := desktop.Serve(ctx, listener, handler); err != nil {
		log.Fatal(err)
	}
}

func generateAdminToken() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

func validateDesktopAddr(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid desktop address: %w", err)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("desktop address must use an explicit loopback host")
	}
	return nil
}
