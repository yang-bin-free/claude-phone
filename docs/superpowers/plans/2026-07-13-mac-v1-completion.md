# Mac V1 Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing macOS prototype into a Finder-launchable, daily-usable Claude Phone desktop client with resilient startup, complete local chat/admin workflows, accurate menu-bar state, and a verifiable `.app` bundle.

**Architecture:** Keep `pkg/engine` UI-independent and keep the browser UI on the loopback-only desktop server. Add a desktop application controller that owns the engine lifecycle and exposes stable handlers even while the engine is unavailable; the native shell consumes controller state for its menu bar, while the WebView consumes the same state through `/desktop/status`.

**Tech Stack:** Go 1.26, Cocoa/WebKit through `webview_go`, `getlantern/systray`, embedded HTML/CSS/vanilla JavaScript, existing Gorilla WebSocket engine protocol.

## Global Constraints

- Mac V1 must be independently useful without Android or iOS.
- The GUI and menu bar remain alive when Claude CLI discovery or engine startup fails.
- The desktop HTTP/admin listener remains loopback-only and the admin token remains in the URL fragment.
- Closing the main window hides it; only Quit terminates the app and engine.
- Local chat continues to use the same WebSocket protocol as mobile clients.
- No Node runtime is required by the shipped app.
- Existing headless `cmd/mac-agent` behavior must remain unchanged.

---

## File Structure

- Create `cmd/mac-app/application.go`: desktop lifecycle controller; starts, stops, and reports the embedded engine.
- Create `cmd/mac-app/application_test.go`: controller startup, degraded-mode, pause/resume, and shutdown tests.
- Create `pkg/desktop/claude.go`: Finder-safe Claude CLI discovery.
- Create `pkg/desktop/claude_test.go`: deterministic CLI discovery tests.
- Modify `pkg/desktop/server.go`: stable desktop status route and unavailable backend behavior.
- Modify `pkg/desktop/native.go`: typed menu-bar state and commands.
- Modify `pkg/desktop/native_darwin.go`: dynamic status, pause/resume, autostart, show/hide, and quit menu items.
- Modify `pkg/adminproto/messages.go`: desktop settings and template administration payloads.
- Modify `pkg/engine/admin.go`: settings/template/autostart administration routes.
- Modify `web/chat/index.html`: startup banner, proper desktop navigation, structured session administration, settings fields.
- Modify `web/chat/chat.js`: desktop view state, keyboard behavior, disabled/offline state, safe reconnect.
- Modify `web/admin/admin.js`: structured cards and complete settings/template/session actions.
- Modify `web/chat/core.css`, `web/chat/desktop.css`, `web/admin/admin.css`: production desktop layout and state styling.
- Create `scripts/AppIcon.svg`: source artwork for the macOS application icon.
- Modify `scripts/build-mac-app.sh`: generate `.icns`, inject version metadata, and verify the bundle.
- Modify `scripts/Info.plist`: icon and version placeholders consumed by the build script.

---

### Task 1: Finder-safe startup and desktop lifecycle controller

**Files:**
- Create: `pkg/desktop/claude.go`
- Create: `pkg/desktop/claude_test.go`
- Create: `cmd/mac-app/application.go`
- Create: `cmd/mac-app/application_test.go`
- Modify: `cmd/mac-app/main.go:21-74`
- Modify: `pkg/desktop/server.go:9-29`
- Modify: `pkg/desktop/server_test.go:9-50`

**Interfaces:**
- Produces: `desktop.ResolveClaudeBinary(requested string) (string, error)`.
- Produces: `application.Start() error`, `application.Pause() error`, `application.Resume() error`, `application.Close() error`, and `application.Status() desktop.AppStatus`.
- Produces: `desktop.HandlerOptions` and `desktop.AppStatus` served as JSON from `GET /desktop/status`.

- [ ] **Step 1: Write deterministic Claude CLI discovery tests**

Add table-driven tests which set `PATH` with `t.Setenv`, create executable fake binaries under `t.TempDir()`, and assert this order: explicit absolute path, current `PATH`, `$HOME/.local/bin/claude`, `/opt/homebrew/bin/claude`, `/usr/local/bin/claude`. Also assert a non-executable file is rejected.

```go
func TestResolveClaudeBinaryUsesExplicitPath(t *testing.T) {
	bin := writeExecutable(t, "claude")
	got, err := ResolveClaudeBinary(bin)
	if err != nil || got != bin {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestResolveClaudeBinaryReportsSearchedLocations(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	_, err := ResolveClaudeBinary("claude")
	if err == nil || !strings.Contains(err.Error(), ".local/bin/claude") {
		t.Fatalf("err=%v", err)
	}
}
```

- [ ] **Step 2: Run the discovery tests and verify red**

Run: `go test ./pkg/desktop -run ResolveClaudeBinary -count=1`

Expected: FAIL because `ResolveClaudeBinary` does not exist.

- [ ] **Step 3: Implement Finder-safe discovery**

Use `exec.LookPath` first, then explicit executable candidates derived from `os.UserHomeDir()`. Return an absolute path after checking regular-file mode and at least one execute bit. The error must list every searched path so the WebView can show actionable setup guidance.

- [ ] **Step 4: Write lifecycle and stable-handler tests**

Construct the application with injected `detectVersion`, `newEngine`, and `listen` functions. Assert that a detection error leaves `AppStatus.Ready == false` while `/` and `/desktop/status` still return 200, `/ws` returns 503, Resume retries startup, Pause closes the engine without closing the desktop listener, and Close is idempotent.

```go
func TestApplicationKeepsDesktopAliveWhenClaudeIsUnavailable(t *testing.T) {
	app := newTestApplication(t, dependencies{
		detectVersion: func(string) (string, error) { return "", errors.New("claude missing") },
	})
	if err := app.Start(); err != nil { t.Fatal(err) }
	status := app.Status()
	if status.Ready || !strings.Contains(status.Error, "claude missing") {
		t.Fatalf("status=%+v", status)
	}
	assertHTTPStatus(t, app.Handler(), "/", http.StatusOK)
	assertHTTPStatus(t, app.Handler(), "/desktop/status", http.StatusOK)
	assertHTTPStatus(t, app.Handler(), "/ws", http.StatusServiceUnavailable)
}
```

- [ ] **Step 5: Run lifecycle tests and verify red**

Run: `go test ./cmd/mac-app ./pkg/desktop -run 'Application|DesktopStatus' -count=1`

Expected: FAIL because the application controller and stable desktop status route do not exist.

- [ ] **Step 6: Implement the lifecycle controller and stable handler**

Move engine ownership out of `main()` into `application`. Keep a single desktop listener for the process lifetime. Guard current engine/chat/admin handlers with `sync.RWMutex`; unavailable handlers return JSON/HTTP 503 without exposing the admin token. `main()` becomes flag parsing, signal context creation, application construction, and `desktop.RunNative`.

- [ ] **Step 7: Verify Task 1**

Run:

```bash
go test ./cmd/mac-app ./pkg/desktop -count=1
go test ./...
go build ./cmd/mac-app
```

Expected: all commands exit 0.

- [ ] **Step 8: Commit Task 1**

```bash
git add cmd/mac-app pkg/desktop
git commit -m "feat(mac): add resilient desktop lifecycle"
```

---

### Task 2: Accurate native menu-bar controls

**Files:**
- Modify: `pkg/desktop/native.go:8-23`
- Modify: `pkg/desktop/native_darwin.go:19-70`
- Modify: `pkg/desktop/native_stub.go`
- Modify: `pkg/desktop/native_test.go`
- Modify: `cmd/mac-app/main.go`
- Modify: `cmd/mac-app/application.go`

**Interfaces:**
- Consumes: Task 1 `application.Status`, `Pause`, `Resume`, and `Close`.
- Produces: `desktop.MenuState { Ready, Paused bool; StatusText string; Devices, Sessions int; Autostart bool }`.
- Produces: `desktop.Commands { Pause, Resume, ToggleAutostart, OpenDiagnostics, Quit func() }`.

- [ ] **Step 1: Add menu state and command contract tests**

Test a pure `menuPresentation(MenuState)` helper so native behavior is verifiable without Cocoa. Assert exact Chinese labels for ready, paused, and failed states, and assert whether Pause or Resume is enabled.

```go
func TestMenuPresentationFailed(t *testing.T) {
	got := menuPresentation(MenuState{StatusText: "找不到 Claude CLI"})
	if got.Status != "引擎异常 · 找不到 Claude CLI" || !got.ResumeEnabled || got.PauseEnabled {
		t.Fatalf("presentation=%+v", got)
	}
}
```

- [ ] **Step 2: Run menu tests and verify red**

Run: `go test ./pkg/desktop -run MenuPresentation -count=1`

Expected: FAIL because `MenuState` and `menuPresentation` do not exist.

- [ ] **Step 3: Implement typed menu presentation and commands**

Replace the static “引擎运行中” item with state-driven text, add read-only device/session counts, add Pause/Resume, add an autostart checkbox backed by `desktop.AutostartEnabled`, retain Show/Hide, add “打开诊断” which dispatches JavaScript to select the admin view, and keep Quit as the only action that cancels the process context.

- [ ] **Step 4: Connect periodic state updates**

The application publishes a buffered state channel and emits immediately after Start/Pause/Resume plus every five seconds while running. Never block the engine on a slow menu consumer; replace the pending buffered state.

- [ ] **Step 5: Verify Task 2**

Run:

```bash
go test ./pkg/desktop ./cmd/mac-app -count=1
go test ./...
go build ./cmd/mac-app
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit Task 2**

```bash
git add cmd/mac-app pkg/desktop
git commit -m "feat(mac): complete menu bar controls"
```

---

### Task 3: Complete local administration workflows

**Files:**
- Modify: `pkg/adminproto/messages.go:4-79`
- Modify: `pkg/engine/admin.go:20-127`
- Modify: `pkg/engine/admin_test.go`
- Modify: `web/chat/index.html:33-47`
- Modify: `web/admin/admin.js`
- Modify: `web/admin/admin.css`

**Interfaces:**
- Produces: `adminproto.UpdateSettingsRequest { DefaultWorkingDir, DefaultPermission string; MaxConcurrentSessions int }`.
- Produces: `adminproto.Template { ID, Label, Prompt string }`.
- Produces: `PATCH /admin/settings`, `POST /admin/templates`, `DELETE /admin/templates/{templateID}`, `POST /admin/autostart`, and the existing `POST /admin/sessions/stop`.

- [ ] **Step 1: Write failing admin API tests**

Add authenticated loopback requests that update runtime settings, add/delete a prompt template, toggle autostart through an injected adapter, and stop a session. Assert invalid permissions, relative working directories, max concurrency outside `1..20`, blank template labels/prompts, and malformed JSON return 400 without mutating persisted files.

```go
func TestAdminUpdatesSettings(t *testing.T) {
	e := newTestEngine(t)
	w := adminRequest(t, e, http.MethodPatch, "/admin/settings", `{
		"defaultWorkingDir":"/tmp/project",
		"defaultPermission":"acceptEdits",
		"maxConcurrentSessions":7
	}`)
	if w.Code != http.StatusNoContent { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }
	got := e.Status()
	if got.DefaultWorkingDir != "/tmp/project" || got.MaxConcurrentSession != 7 {
		t.Fatalf("status=%+v", got)
	}
}
```

- [ ] **Step 2: Run admin tests and verify red**

Run: `go test ./pkg/engine -run 'Admin.*(Settings|Template|Autostart|Stop)' -count=1`

Expected: FAIL because the routes and payloads are absent.

- [ ] **Step 3: Implement typed admin routes**

Reuse the existing runtime config and template stores; do not create a second persistence format. Validate all request bodies before writing. Keep the current loopback plus bearer-token wrapper around every new route. Inject autostart operations into `AdminHandler` instead of importing Darwin-only code into `pkg/engine`.

- [ ] **Step 4: Replace raw session JSON with structured controls**

Render one session row per `SessionSnapshot` with name, health, owner, subscriber count, idle duration, and a Stop button. Add editable settings and templates sections. Every mutation displays an inline success/error result and refreshes the snapshot; no rejected promise may remain unhandled.

- [ ] **Step 5: Verify Task 3**

Run:

```bash
go test ./pkg/engine -count=1
node --check web/admin/admin.js
go test ./...
```

Expected: all commands exit 0.

- [ ] **Step 6: Commit Task 3**

```bash
git add pkg/adminproto pkg/engine web/chat/index.html web/admin
git commit -m "feat(mac): complete desktop administration"
```

---

### Task 4: Make desktop chat a coherent daily workflow

**Files:**
- Modify: `web/chat/index.html:13-51`
- Modify: `web/chat/chat.js:1-225`
- Modify: `web/chat/core.css`
- Modify: `web/chat/desktop.css`
- Modify: `web/admin/admin.css`
- Modify: `web/assets_test.go`

**Interfaces:**
- Consumes: `GET /desktop/status`, existing WebSocket protocol, and Task 3 admin routes.
- Produces: explicit `showChat`, `showAdmin`, `setComposerEnabled`, and `showBanner` UI functions exposed only inside the chat module.

- [ ] **Step 1: Add asset contract tests for desktop states**

Extend `web/assets_test.go` to assert unique IDs for `startup-banner`, `chat-view`, `admin-view`, `session-list`, `composer`, and `admin-sessions`; assert all local asset URLs resolve through `pkg/desktop.NewHandler`; assert the HTML does not load `mobile.css` when served to the desktop host.

- [ ] **Step 2: Run asset tests and verify red**

Run: `go test ./web ./pkg/desktop -count=1`

Expected: FAIL because the startup banner is absent and desktop still loads the mobile stylesheet.

- [ ] **Step 3: Implement explicit view navigation**

Selecting or creating a session always calls `showChat()`. Opening diagnostics calls `showAdmin()`. “New session” focuses project selection before creation; it must not silently create with an accidental directory. Keep `Cmd+Enter` to send, add `Cmd+N` to start the new-session flow, and add `Cmd+,` to open management.

- [ ] **Step 4: Implement startup/offline/error states**

Fetch `/desktop/status` before opening the WebSocket. If the engine is unavailable, show the exact actionable error, disable session creation and composer submission, and keep Management accessible. WebSocket reconnect uses one timer, clears the previous socket handlers, and never creates parallel reconnect loops. Invalid incoming JSON is shown as a recoverable protocol error rather than terminating `onmessage`.

- [ ] **Step 5: Polish the desktop layout without changing mobile behavior**

Desktop uses a 248px sidebar, 720px readable message column, visible focus rings, selectable/copyable assistant output, compact structured tool cards, empty/loading/error states, and responsive behavior down to an 840×600 window. Mobile rules stay scoped under `body.mobile`; desktop never imports `mobile.css`.

- [ ] **Step 6: Verify Task 4**

Run:

```bash
node --check web/chat/chat.js
node --check web/admin/admin.js
go test ./web ./pkg/desktop -count=1
go test ./...
```

Expected: all commands exit 0.

- [ ] **Step 7: Commit Task 4**

```bash
git add web pkg/desktop
git commit -m "feat(mac): finish desktop chat workflow"
```

---

### Task 5: Produce and verify a distributable Mac application

**Files:**
- Create: `scripts/AppIcon.svg`
- Modify: `scripts/build-mac-app.sh:1-16`
- Modify: `scripts/Info.plist:1-18`
- Modify: `scripts/package-release.sh`
- Modify: `Makefile`
- Modify: `.github/workflows/ci.yml`
- Modify: `README.md`
- Modify: `docs/superpowers/plans/2026-07-08-project-roadmap.md`

**Interfaces:**
- Consumes: environment variable `VERSION`, defaulting to `0.1.0-dev`.
- Produces: `build/Claude Phone.app` with `Contents/MacOS/claude-phone`, `Contents/Resources/AppIcon.icns`, versioned `Info.plist`, licenses, and no external web assets.

- [ ] **Step 1: Add bundle verification before changing the builder**

Create a `verify-mac-app` Make target which asserts the executable, icon, plist identifiers, minimum macOS version, embedded license files, and absence of absolute workspace paths. It must also run `codesign --verify --deep --strict` only when `codesign -dv` reports a signature.

- [ ] **Step 2: Run verification and confirm the current bundle fails**

Run: `make mac-app verify-mac-app`

Expected: FAIL because `AppIcon.icns` and `CFBundleIconFile` are absent.

- [ ] **Step 3: Add icon generation and version injection**

Generate the standard 16, 32, 128, 256, 512, and 1024 pixel iconset from `scripts/AppIcon.svg` using `sips`, convert with `iconutil`, copy to bundle resources, and set `CFBundleIconFile=AppIcon`. Replace plist version placeholders with the sanitized `VERSION`; reject values outside `[0-9A-Za-z][0-9A-Za-z.-]*`.

- [ ] **Step 4: Add CI bundle build and verification**

On the macOS CI job, run `VERSION=0.1.0-ci make mac-app verify-mac-app` after Go tests. Keep signing/notarization out of pull-request CI because credentials are intentionally absent.

- [ ] **Step 5: Run complete automated verification**

Run:

```bash
make verify
VERSION=0.1.0-dev make mac-app verify-mac-app
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 6: Perform the real macOS acceptance pass**

Launch `build/Claude Phone.app` from Finder and record pass/fail for these exact checks:

1. Window appears without Terminal-provided `PATH`.
2. Missing Claude CLI produces an actionable in-window diagnostic instead of silent exit.
3. A configured project can create a session and receive streamed fake-Claude output.
4. Closing the window hides it while the menu bar remains.
5. Show/Hide, Pause/Resume, Diagnostics, Autostart, and Quit menu actions work.
6. Relaunch restores persisted projects, templates, permissions, and history.
7. Quit stops the Claude child process and releases `caffeinate`.

- [ ] **Step 7: Update status documentation using observed results only**

Mark Mac V1 complete only after all seven acceptance checks pass. Document any signing/notarization requirement separately from functional completeness.

- [ ] **Step 8: Commit Task 5**

```bash
git add scripts Makefile .github README.md docs
git commit -m "build(mac): verify distributable desktop app"
```

---

## Final Verification

Run:

```bash
make verify
VERSION=0.1.0-dev make mac-app verify-mac-app
git status --short
```

Expected:

- `make verify` exits 0.
- The `.app` bundle verification exits 0.
- `git status --short` is empty after the final commit.
- The seven-item real macOS acceptance pass is recorded with no failures.

