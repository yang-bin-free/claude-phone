# Third-party licenses

Claude Phone depends on the following major third-party components. The exact
module versions are recorded in `go.mod` and `go.sum`.

| Component | License | Copyright / source |
|---|---|---|
| Tailscale | BSD-3-Clause | Copyright (c) 2020 Tailscale Inc & contributors; https://github.com/tailscale/tailscale |
| wireguard-go | MIT | https://github.com/WireGuard/wireguard-go |
| Gorilla WebSocket | BSD-2-Clause | Copyright (c) 2013 The Gorilla WebSocket Authors; https://github.com/gorilla/websocket |
| webview / webview_go | MIT | Copyright (c) 2017 Serge Zaitsev and (c) 2020 webview; https://github.com/webview/webview |
| getlantern/systray | Apache-2.0 | https://github.com/getlantern/systray |
| Go mobile | BSD-3-Clause | The Go Authors; https://go.googlesource.com/mobile |

Each dependency's complete license text is distributed in its source module.
Binary release archives of Claude Phone must include `LICENSE`, `NOTICE`, and
this file.
