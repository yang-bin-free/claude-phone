//go:build darwin

package desktop

import (
	"context"

	webview "github.com/webview/webview_go"
)

func runNative(ctx context.Context, pageURL string, commands Commands) error {
	window := webview.New(false)
	defer window.Destroy()
	window.SetTitle("Claude Phone")
	window.SetSize(1180, 760, webview.HintNone)
	window.Navigate(pageURL)

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			window.Terminate()
		case <-done:
		}
	}()
	window.Run()
	close(done)
	if commands.Quit != nil {
		commands.Quit()
	}
	return nil
}
