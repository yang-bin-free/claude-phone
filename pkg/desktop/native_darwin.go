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
		status := systray.AddMenuItem("引擎运行中", "Claude Phone engine status")
		status.Disable()
		systray.AddSeparator()
		show := systray.AddMenuItem("打开主窗口", "Show Claude Phone")
		hide := systray.AddMenuItem("隐藏主窗口", "Hide Claude Phone")
		systray.AddSeparator()
		quit := systray.AddMenuItem("退出", "Quit Claude Phone")

		go func() {
			for {
				select {
				case <-show.ClickedCh:
					window.Dispatch(func() { C.cpShowWindow(unsafe.Pointer(window.Window())) })
				case <-hide.ClickedCh:
					window.Dispatch(func() { C.cpHideWindow(unsafe.Pointer(window.Window())) })
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
			window.Terminate()
			systray.Quit()
		case <-done:
		}
	}()
	window.Run()
	close(done)
	return nil
}
