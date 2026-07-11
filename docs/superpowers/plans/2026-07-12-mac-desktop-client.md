# Mac Desktop Client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a menu-bar-resident macOS client that embeds the existing engine, provides a shared chat UI, and exposes loopback-only administration.

**Architecture:** Keep `pkg/engine` UI-independent. Add a typed local admin HTTP API protected by both loopback-source validation and a process-generated bearer token. Serve embedded chat/admin assets from `pkg/desktop`; `cmd/mac-app` starts the engine and desktop host, while a Darwin-only shell adds native WebView and menu-bar behavior without breaking non-macOS tests.

**Tech Stack:** Go 1.26, `net/http`, `embed`, existing Gorilla WebSocket protocol, Darwin WebKit/menu-bar adapter, HTML/CSS/vanilla JavaScript.

## Global Constraints

- The GUI is an engine client; chat continues to use the existing WebSocket protocol.
- Admin operations require a loopback connection and a process-local bearer token.
- Mobile/device tokens never authorize admin operations.
- Closing the window does not stop the engine; Quit performs graceful shutdown.
- No iOS changes and no Node runtime.

---

### Task 1: Loopback-only admin API

**Files:**
- Create: `pkg/adminproto/messages.go`
- Create: `pkg/engine/admin.go`
- Create: `pkg/engine/admin_test.go`
- Modify: `pkg/engine/engine.go`

**Interfaces:**
- Produces: `AdminHandler(token string) http.Handler`, `adminproto.Snapshot`, `adminproto.StopSessionRequest`.

- [ ] Write tests proving missing tokens, wrong tokens, and non-loopback peers receive HTTP 403; valid loopback requests receive a status snapshot; valid stop requests remove the session.
- [ ] Run `go test ./pkg/engine -run Admin -count=1` and verify the tests fail because `AdminHandler` does not exist.
- [ ] Implement JSON routes `GET /admin/status` and `POST /admin/sessions/stop` with constant-time token comparison and loopback address validation.
- [ ] Run `go test ./pkg/engine -run Admin -count=1` and verify it passes.
- [ ] Run `go test ./...`.

### Task 2: Embedded desktop web host

**Files:**
- Create: `pkg/desktop/server.go`
- Create: `pkg/desktop/server_test.go`
- Create: `web/assets.go`
- Create: `web/chat/index.html`
- Create: `web/chat/chat.js`
- Create: `web/chat/core.css`
- Create: `web/chat/desktop.css`
- Create: `web/admin/admin.js`
- Create: `web/admin/admin.css`

**Interfaces:**
- Consumes: existing `/ws` chat protocol and Task 1 `/admin/*` routes.
- Produces: `desktop.NewHandler(engineHandler, adminHandler http.Handler) http.Handler`.

- [ ] Write HTTP tests asserting `/` serves the desktop shell, `/assets/chat.js` is JavaScript, `/ws` delegates to the engine handler, and `/admin/status` delegates to the admin handler.
- [ ] Run `go test ./pkg/desktop -count=1` and verify failure because the package is absent.
- [ ] Implement an embedded responsive two-pane shell: session navigation and streaming chat on the left/main pane, engine/session diagnostics on the admin pane; JavaScript reads the in-process token from the URL fragment (never sent in HTTP requests) and reconnects WebSocket with bounded backoff.
- [ ] Run `go test ./pkg/desktop -count=1` and `go test ./...`.

### Task 3: Desktop process entry point

**Files:**
- Create: `cmd/mac-app/main.go`
- Create: `cmd/mac-app/main_test.go`
- Create: `pkg/desktop/runtime.go`

**Interfaces:**
- Consumes: `engine.Engine`, `desktop.NewHandler`.
- Produces: one process with separate loopback desktop/admin listener and optional tsnet listener; random 256-bit admin token.

- [ ] Write tests for random token length, loopback desktop address validation, and graceful cancellation.
- [ ] Run `go test ./cmd/mac-app -count=1` and verify failure.
- [ ] Implement configuration parsing, token generation, engine startup, desktop server startup, signal handling, and graceful shutdown.
- [ ] Run `go test ./cmd/mac-app -count=1`, `go test ./...`, and `go build ./cmd/mac-app`.

### Task 4: Darwin native shell

**Files:**
- Create: `pkg/desktop/native_darwin.go`
- Create: `pkg/desktop/native_stub.go`
- Modify: `cmd/mac-app/main.go`

**Interfaces:**
- Produces: `desktop.RunNative(ctx context.Context, url string, commands Commands) error`; `Commands` exposes Show, Pause, Resume, Pair, Quit callbacks.

- [ ] Add a stub test proving non-Darwin builds return `desktop.ErrNativeUnsupported` without affecting the server.
- [ ] Implement the Darwin adapter behind `//go:build darwin`; window close hides the window while the menu-bar item remains active, and Quit cancels the process context.
- [ ] Run `go test ./...`, `GOOS=darwin go build ./cmd/mac-app`, and the local macOS binary smoke test.

### Task 5: Documentation and release verification

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-08-project-roadmap.md`

**Interfaces:**
- Documents the exact build, headless mode, GUI mode, and P0c/P2 status.

- [ ] Update project structure, Mac commands, security boundary, and phase table with implemented behavior only.
- [ ] Run placeholder and secret scans: `rg -n 'TBD|TODO|tskey-auth-[A-Za-z0-9_-]{8,}' README.md docs pkg cmd web` and inspect every match.
- [ ] Run `gofmt`, `go test ./...`, both command builds, Android AAR contract test, and Android debug APK build.
- [ ] Review `git diff --check`, commit scoped changes, and push `master`.
