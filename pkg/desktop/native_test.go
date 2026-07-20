package desktop

import (
	"net/url"
	"strings"
	"testing"
)

func TestURLWithAdminTokenUsesFragment(t *testing.T) {
	got, err := URLWithAdminToken("http://127.0.0.1:9877/", "secret value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "token=secret+value" {
		t.Fatalf("url=%q", got)
	}
}

func TestDirectoryPickerBootstrapExposesNarrowBridge(t *testing.T) {
	if !strings.Contains(directoryPickerBootstrap, "window.codeAfarNative") || !strings.Contains(directoryPickerBootstrap, directoryPickerBinding) {
		t.Fatalf("bootstrap=%q binding=%q", directoryPickerBootstrap, directoryPickerBinding)
	}
	if strings.Contains(directoryPickerBootstrap, "fetch") {
		t.Fatalf("native bridge should only expose directory selection: %q", directoryPickerBootstrap)
	}
}

func TestMenuPresentationRunning(t *testing.T) {
	got := menuPresentation(MenuState{Ready: true, Devices: 2, Sessions: 3, Autostart: true})
	if got.Status != "引擎运行中" || got.Devices != "在线设备：2" || got.Sessions != "活跃会话：3" {
		t.Fatalf("presentation=%+v", got)
	}
	if !got.PauseEnabled || got.ResumeEnabled || !got.Autostart {
		t.Fatalf("presentation=%+v", got)
	}
}

func TestMenuPresentationPaused(t *testing.T) {
	got := menuPresentation(MenuState{Paused: true})
	if got.Status != "引擎已暂停" || got.PauseEnabled || !got.ResumeEnabled {
		t.Fatalf("presentation=%+v", got)
	}
}

func TestMenuPresentationFailed(t *testing.T) {
	got := menuPresentation(MenuState{StatusText: "找不到 Claude CLI"})
	if got.Status != "引擎异常 · 找不到 Claude CLI" || !got.ResumeEnabled || got.PauseEnabled {
		t.Fatalf("presentation=%+v", got)
	}
}

func TestShutdownNativeClosesResourcesBeforeTerminatingWindow(t *testing.T) {
	var events []string
	shutdownNative(Commands{Quit: func() { events = append(events, "close") }}, func() {
		events = append(events, "terminate")
	})
	if len(events) != 2 || events[0] != "close" || events[1] != "terminate" {
		t.Fatalf("shutdown order=%v", events)
	}
}

func TestPrepareNativeShellRegistersStatusItemBeforeWindowCreation(t *testing.T) {
	var events []string
	prepareNativeShell(func() { events = append(events, "register") }, func() {
		events = append(events, "window")
	})
	if len(events) != 2 || events[0] != "register" || events[1] != "window" {
		t.Fatalf("native startup order=%v", events)
	}
}
