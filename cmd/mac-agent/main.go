package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yang-bin-free/claude-phone/pkg/engine"
	"github.com/yang-bin-free/claude-phone/pkg/product"
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
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	cfg.DataDir, _, err = product.ResolveDataDir(home, cfg.DataDir)
	if err != nil {
		log.Fatalf("CodeAfar data migration failed: %v", err)
	}
	version, err := engine.DetectClaudeVersion(cfg.ClaudeBin)
	if err != nil {
		log.Fatalf("Claude CLI check failed: %v", err)
	}
	cfg.ClaudeVersion = version

	e := engine.New(cfg)
	if network.Enabled() {
		log.Printf("codeafar-agent joining tailnet as %s and listening on %s", network.Hostname, cfg.Addr)
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

	log.Printf("codeafar-agent listening locally on %s", cfg.Addr)
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
	fs := flag.NewFlagSet("codeafar-agent", flag.ContinueOnError)
	addr := fs.String("addr", engine.DefaultAddr, "HTTP/WebSocket listen address")
	claudeBin := fs.String("claude-bin", "claude", "Claude CLI binary")
	workdir := fs.String("workdir", ".", "default Claude working directory")
	permission := fs.String("permission", "default", "default Claude permission mode")
	dataDir := fs.String("data-dir", "", "CodeAfar configuration directory")
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
	dataDir := fs.String("data-dir", "", "CodeAfar configuration directory")
	name := fs.String("name", "Android", "device display name")
	_ = fs.Parse(args)

	credential, err := engine.GenerateDeviceCredential(*dataDir, *name)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("device-name: %s\n", credential.Device.Name)
	fmt.Printf("device-token: %s\n", credential.DeviceToken)
}

func statusURL(addr string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/") + "/status"
	}
	return "http://" + strings.TrimRight(addr, "/") + "/status"
}
