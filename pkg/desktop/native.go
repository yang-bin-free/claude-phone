package desktop

import (
	"context"
	"fmt"
	"net/url"
)

type Commands struct {
	States          <-chan MenuState
	Pause           func() error
	Resume          func() error
	ToggleAutostart func() error
	OpenDiagnostics func()
	Quit            func()
}

type MenuState struct {
	Ready      bool
	Paused     bool
	StatusText string
	Devices    int
	Sessions   int
	Autostart  bool
}

type menuView struct {
	Status        string
	Devices       string
	Sessions      string
	PauseEnabled  bool
	ResumeEnabled bool
	Autostart     bool
}

func menuPresentation(state MenuState) menuView {
	view := menuView{
		Devices:   fmt.Sprintf("在线设备：%d", state.Devices),
		Sessions:  fmt.Sprintf("活跃会话：%d", state.Sessions),
		Autostart: state.Autostart,
	}
	switch {
	case state.Ready:
		view.Status = "引擎运行中"
		view.PauseEnabled = true
	case state.Paused:
		view.Status = "引擎已暂停"
		view.ResumeEnabled = true
	case state.StatusText != "":
		view.Status = "引擎异常 · " + state.StatusText
		view.ResumeEnabled = true
	default:
		view.Status = "引擎启动中"
	}
	return view
}

func URLWithAdminToken(baseURL, token string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	fragment := url.Values{"token": []string{token}}
	parsed.Fragment = fragment.Encode()
	return parsed.String(), nil
}

func RunNative(ctx context.Context, pageURL string, commands Commands) error {
	return runNative(ctx, pageURL, commands)
}

func shutdownNative(commands Commands, terminate func()) {
	if commands.Quit != nil {
		commands.Quit()
	}
	terminate()
}

func prepareNativeShell(registerStatusItem, createWindow func()) {
	registerStatusItem()
	createWindow()
}
