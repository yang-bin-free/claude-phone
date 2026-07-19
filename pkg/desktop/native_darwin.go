//go:build darwin

package desktop

/*
#cgo LDFLAGS: -framework Cocoa
#include "native_darwin.h"
*/
import "C"

import (
	"context"
	"unsafe"

	"github.com/getlantern/systray"
	webview "github.com/webview/webview_go"
)

func runNative(ctx context.Context, pageURL string, commands Commands) error {
	window := webview.New(false)
	defer window.Destroy()
	window.SetTitle("Claude Phone")
	window.SetSize(1180, 760, webview.HintNone)
	window.Navigate(pageURL)
	C.cpConfigureWindow(unsafe.Pointer(window.Window()))

	done := make(chan struct{})
	systray.Register(func() {
		systray.SetTitle("CP")
		systray.SetTooltip("Claude Phone")
		status := systray.AddMenuItem("引擎启动中", "Claude Phone engine status")
		status.Disable()
		devices := systray.AddMenuItem("在线设备：0", "Connected devices")
		devices.Disable()
		sessions := systray.AddMenuItem("活跃会话：0", "Active sessions")
		sessions.Disable()
		systray.AddSeparator()
		show := systray.AddMenuItem("打开主窗口", "Show Claude Phone")
		hide := systray.AddMenuItem("隐藏主窗口", "Hide Claude Phone")
		diagnostics := systray.AddMenuItem("打开诊断", "Open diagnostics")
		systray.AddSeparator()
		pause := systray.AddMenuItem("暂停引擎", "Pause Claude Phone engine")
		resume := systray.AddMenuItem("恢复引擎", "Resume Claude Phone engine")
		autostart := systray.AddMenuItemCheckbox("开机自启", "Start Claude Phone at login", false)
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出", "Quit Claude Phone")
		applyState := func(state MenuState) {
			view := menuPresentation(state)
			status.SetTitle(view.Status)
			devices.SetTitle(view.Devices)
			sessions.SetTitle(view.Sessions)
			if view.PauseEnabled {
				pause.Enable()
			} else {
				pause.Disable()
			}
			if view.ResumeEnabled {
				resume.Enable()
			} else {
				resume.Disable()
			}
			if view.Autostart {
				autostart.Check()
			} else {
				autostart.Uncheck()
			}
		}
		applyState(MenuState{})

		go func() {
			for {
				select {
				case state, ok := <-commands.States:
					if !ok {
						return
					}
					applyState(state)
				case <-show.ClickedCh:
					window.Dispatch(func() { C.cpShowWindow(unsafe.Pointer(window.Window())) })
				case <-hide.ClickedCh:
					window.Dispatch(func() { C.cpHideWindow(unsafe.Pointer(window.Window())) })
				case <-diagnostics.ClickedCh:
					window.Dispatch(func() {
						C.cpShowWindow(unsafe.Pointer(window.Window()))
						window.Eval("window.claudePhone?.showAdmin?.()")
					})
					if commands.OpenDiagnostics != nil {
						commands.OpenDiagnostics()
					}
				case <-pause.ClickedCh:
					if commands.Pause != nil {
						if err := commands.Pause(); err != nil {
							status.SetTitle("暂停失败 · " + err.Error())
						}
					}
				case <-resume.ClickedCh:
					if commands.Resume != nil {
						if err := commands.Resume(); err != nil {
							status.SetTitle("恢复失败 · " + err.Error())
						}
					}
				case <-autostart.ClickedCh:
					if commands.ToggleAutostart != nil {
						if err := commands.ToggleAutostart(); err != nil {
							status.SetTitle("自启设置失败 · " + err.Error())
						}
					}
				case <-quit.ClickedCh:
					systray.Quit()
				case <-done:
					return
				}
			}
		}()
	}, func() {
		if commands.Quit != nil {
			commands.Quit()
		}
	})

	go func() {
		select {
		case <-ctx.Done():
			shutdownNative(commands, func() {
				window.Terminate()
				systray.Quit()
			})
		case <-done:
		}
	}()
	window.Run()
	close(done)
	return nil
}
