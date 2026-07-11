package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

func TestGenerateDeviceCredential(t *testing.T) {
	credential, err := engine.GenerateDeviceCredential(t.TempDir(), "Pixel")
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if !strings.HasPrefix(credential.DeviceToken, "dt_") {
		t.Fatalf("key prefix mismatch: %q", credential.DeviceToken)
	}
	if credential.Device.Name != "Pixel" {
		t.Fatalf("name=%q", credential.Device.Name)
	}
}

func TestParseServeConfig_LocalByDefault(t *testing.T) {
	cfg, network, err := parseServeConfig([]string{"--addr", "127.0.0.1:9999", "--workdir", "/work"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9999" || cfg.DefaultWorkingDir != "/work" {
		t.Fatalf("engine config: %+v", cfg)
	}
	if network.Enabled() {
		t.Fatalf("tsnet unexpectedly enabled: %+v", network)
	}
}

func TestParseServeConfig_TSNet(t *testing.T) {
	args := []string{
		"--addr", ":9876",
		"--tsnet-dir", "/state",
		"--tsnet-hostname", "claude-mac",
		"--tsnet-auth-key", "tskey-auth-test",
		"--tsnet-control-url", "https://control.example.test",
	}
	_, got, err := parseServeConfig(args)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := tsnetOptions{
		Dir:        "/state",
		Hostname:   "claude-mac",
		AuthKey:    "tskey-auth-test",
		ControlURL: "https://control.example.test",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tsnet options: got %+v want %+v", got, want)
	}
}
