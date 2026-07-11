package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "status":
			runStatus(os.Args[2:])
			return
		case "key":
			runKey(os.Args[2:])
			return
		case "serve":
			runServe(os.Args[2:])
			return
		}
	}
	runServe(os.Args[1:])
}

func runServe(args []string) {
	cfg, network, err := parseServeConfig(args)
	if err != nil {
		log.Fatal(err)
	}

	e := engine.New(cfg)
	if network.Enabled() {
		log.Printf("claude-phone-agent joining tailnet as %s and listening on %s", network.Hostname, cfg.Addr)
		if err := e.ServeTSNet(engine.TSNetConfig{
			Hostname:   network.Hostname,
			Dir:        network.Dir,
			AuthKey:    network.AuthKey,
			ControlURL: network.ControlURL,
		}, cfg.Addr); err != nil {
			log.Fatal(err)
		}
		return
	}

	log.Printf("claude-phone-agent listening locally on %s", cfg.Addr)
	if err := e.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

type tsnetOptions struct {
	Dir        string
	Hostname   string
	AuthKey    string
	ControlURL string
}

func (o tsnetOptions) Enabled() bool { return o.Dir != "" }

func parseServeConfig(args []string) (engine.Config, tsnetOptions, error) {
	fs := flag.NewFlagSet("claude-phone-agent", flag.ContinueOnError)
	addr := fs.String("addr", engine.DefaultAddr, "HTTP/WebSocket listen address")
	claudeBin := fs.String("claude-bin", "claude", "Claude CLI binary")
	workdir := fs.String("workdir", ".", "default Claude working directory")
	permission := fs.String("permission", "default", "default Claude permission mode")
	dataDir := fs.String("data-dir", "", "Claude Phone configuration directory")
	tsnetDir := fs.String("tsnet-dir", "", "persistent tsnet state directory (enables tailnet listener)")
	tsnetHostname := fs.String("tsnet-hostname", "claude-mac", "tailnet hostname")
	tsnetAuthKey := fs.String("tsnet-auth-key", os.Getenv("TS_AUTHKEY"), "Tailscale auth key (or TS_AUTHKEY)")
	tsnetControlURL := fs.String("tsnet-control-url", "", "optional Tailscale-compatible control server URL")
	if err := fs.Parse(args); err != nil {
		return engine.Config{}, tsnetOptions{}, err
	}

	return engine.Config{
			Addr:              *addr,
			ClaudeBin:         *claudeBin,
			DefaultWorkingDir: *workdir,
			DefaultPermission: *permission,
			DataDir:           *dataDir,
		}, tsnetOptions{
			Dir:        *tsnetDir,
			Hostname:   *tsnetHostname,
			AuthKey:    *tsnetAuthKey,
			ControlURL: *tsnetControlURL,
		}, nil
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	addr := fs.String("addr", engine.DefaultAddr, "agent HTTP listen address")
	timeout := fs.Duration("timeout", 3*time.Second, "HTTP request timeout")
	_ = fs.Parse(args)

	client := &http.Client{Timeout: *timeout}
	resp, err := client.Get(statusURL(*addr))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("status endpoint returned %s", resp.Status)
	}
	var report engine.StatusReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		log.Fatal(err)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(out))
}

func runKey(args []string) {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	ttl := fs.Duration("ttl", time.Hour, "key lifetime")
	_ = fs.Parse(args)

	key, err := generatePairingKey()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("pairing-key: %s\n", key)
	fmt.Printf("expires-at: %s\n", time.Now().Add(*ttl).UTC().Format(time.RFC3339))
}

func generatePairingKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "pk_" + hex.EncodeToString(b[:]), nil
}

func statusURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/") + "/status"
	}
	return "http://" + strings.TrimRight(addr, "/") + "/status"
}
