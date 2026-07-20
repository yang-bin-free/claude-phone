# CodeAfar Mac Session and Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver an installable `CodeAfar.app` whose new-session flow selects a real Mac folder and creates exactly one Claude Code session only when the first prompt is sent, while preserving existing user data and introducing a provider-neutral backend boundary.

**Architecture:** Keep the existing Go engine, WebSocket protocol, embedded Web UI, and Claude CLI process, but put provider metadata and process construction behind a registry. Model the Web UI as an unpersisted draft until `session_created`, and expose the macOS folder picker through a narrow WebView binding. Rename user-visible artifacts and migrate the default data directory without renaming the repository or Go module.

**Tech Stack:** Go 1.22+, Gorilla WebSocket, `webview_go`, Cocoa `NSOpenPanel`, vanilla JavaScript/CSS, Bash/macOS packaging, Go/Node contract tests.

## Global Constraints

- Product name is `CodeAfar`; tagline is `Run locally. Code from anywhere.`
- Build `CodeAfar.app`, `codeafar`, and `codeafar-agent`; use `~/.codeafar` as the new default data directory.
- Do not rename the GitHub repository, workspace directory, or Go module path in this delivery.
- If `~/.codeafar` is absent and `~/.claude-phone` exists, migrate it once; never overwrite an existing destination and never migrate an explicit `--data-dir`.
- Missing `provider` means `claude`; unknown providers fail explicitly.
- Clicking New Session or pressing Command-N enters a draft and must not create a backend session.
- The first non-empty prompt creates the session, waits for `session_created`, then sends text exactly once.
- Session titles are derived locally from normalized first-prompt text, capped at 32 Unicode characters; no model call is allowed.
- Existing-session folder and provider are immutable; permission may change and applies only to later work.
- Only the Claude provider is implemented in V1; the UI and protocol must not hard-code a future Codex permission mapping.

---

## File Structure

- `pkg/product/product.go`: canonical brand, binary, bundle, and data-directory constants.
- `pkg/product/migrate.go`: safe one-time legacy data migration.
- `pkg/provider/provider.go`: provider descriptors, process interface, adapter interface, registry.
- `pkg/provider/claude.go`: Claude Code descriptor and process creation.
- `pkg/protocol/messages.go`: provider/session configuration and idempotency fields.
- `pkg/session/session.go`, `pkg/session/manager.go`: persisted provider/model/permission metadata.
- `pkg/engine/history.go`: atomic session metadata updates.
- `pkg/engine/engine.go`, `pkg/engine/wsserver.go`: provider registry, idempotent creation, permission update.
- `pkg/desktop/native_darwin.{go,h,m}`: WebView binding and `NSOpenPanel`.
- `pkg/desktop/server.go`, `pkg/desktop/native.go`: locally authorize a picked project and return it to the page.
- `web/chat/index.html`, `web/chat/chat.js`, `web/chat/core.css`, `web/chat/desktop.css`, `web/chat/mobile.css`: draft UI and state machine.
- `scripts/*.sh`, `scripts/Info.plist`, `Makefile`: CodeAfar artifacts, install/verification paths.
- Platform plist/manifest/view files: user-visible mobile brand only.

### Task 1: Product Identity and Safe Data Migration

**Files:**
- Create: `pkg/product/product.go`
- Create: `pkg/product/migrate.go`
- Create: `pkg/product/migrate_test.go`
- Modify: `pkg/engine/config.go`
- Modify: `cmd/mac-app/main.go`
- Modify: `cmd/mac-agent/main.go`

**Interfaces:**
- Produces: `product.Name`, `product.Tagline`, `product.DataDirName`, and `product.DefaultDataDir(home string) string`.
- Produces: `product.ResolveDataDir(home, explicit string) (path string, migrated bool, err error)`.

- [ ] **Step 1: Write migration tests**

```go
func TestResolveDataDirMigratesLegacyDirectory(t *testing.T) {
    home := t.TempDir()
    legacy := filepath.Join(home, ".claude-phone")
    require.NoError(t, os.MkdirAll(legacy, 0o700))
    require.NoError(t, os.WriteFile(filepath.Join(legacy, "projects.yaml"), []byte("projects: []\n"), 0o600))
    got, migrated, err := ResolveDataDir(home, "")
    require.NoError(t, err)
    assert.True(t, migrated)
    assert.Equal(t, filepath.Join(home, ".codeafar"), got)
    _, oldErr := os.Stat(legacy)
    assert.ErrorIs(t, oldErr, os.ErrNotExist)
}

func TestResolveDataDirNeverOverwritesDestinationOrMigratesExplicitPath(t *testing.T) {
    // Existing .codeafar wins; an explicit path is returned unchanged.
}
```

- [ ] **Step 2: Verify the tests fail**

Run: `go test ./pkg/product -run TestResolveDataDir -v`

Expected: FAIL because `ResolveDataDir` does not exist.

- [ ] **Step 3: Implement constants and rename-based migration**

```go
const (
    Name = "CodeAfar"
    Tagline = "Run locally. Code from anywhere."
    DataDirName = ".codeafar"
    LegacyDataDirName = ".claude-phone"
)

func ResolveDataDir(home, explicit string) (string, bool, error) {
    if explicit != "" { return explicit, false, nil }
    current := filepath.Join(home, DataDirName)
    legacy := filepath.Join(home, LegacyDataDirName)
    if _, err := os.Stat(current); err == nil { return current, false, nil }
    if _, err := os.Stat(legacy); errors.Is(err, os.ErrNotExist) { return current, false, nil } else if err != nil { return "", false, err }
    if err := os.Rename(legacy, current); err != nil { return "", false, fmt.Errorf("migrate CodeAfar data: %w", err) }
    return current, true, nil
}
```

Wire default startup through `ResolveDataDir`; keep explicit flags untouched.

- [ ] **Step 4: Run focused and configuration tests**

Run: `go test ./pkg/product ./pkg/engine ./cmd/mac-app ./cmd/mac-agent`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/product pkg/engine/config.go cmd/mac-app/main.go cmd/mac-agent/main.go
git commit -m "feat: migrate local data to CodeAfar"
```

### Task 2: Provider-Neutral Protocol and Persisted Session Metadata

**Files:**
- Create: `pkg/provider/provider.go`
- Create: `pkg/provider/claude.go`
- Create: `pkg/provider/provider_test.go`
- Modify: `pkg/protocol/messages.go`
- Modify: `pkg/protocol/messages_test.go`
- Modify: `pkg/session/session.go`
- Modify: `pkg/session/manager.go`
- Modify: `pkg/session/manager_test.go`
- Modify: `pkg/engine/history.go`
- Modify: `pkg/engine/persistence_test.go`

**Interfaces:**
- Produces: `provider.Process` with `OnOutput(session.OutputFunc)`, `Start() error`, `Send(string) error`, and `Stop() error`.
- Produces: `provider.Adapter` with `Descriptor() Descriptor` and `NewProcess(SessionConfig) Process`.
- Produces: `provider.Registry.Get(id string) (Adapter, bool)`; an empty ID is normalized by `provider.NormalizeID("") == "claude"`.
- Produces: `session.Config{Name, Cwd, Owner, Provider, Model, Permission string}` and `Manager.Create(Config)`.

- [ ] **Step 1: Write protocol, registry, and persistence tests**

```go
func TestNormalizeIDDefaultsLegacySessionsToClaude(t *testing.T) {
    assert.Equal(t, "claude", provider.NormalizeID(""))
}

func TestUnknownProviderIsNotRegistered(t *testing.T) {
    _, ok := provider.NewRegistry(provider.NewClaudeAdapter(...)).Get("codex")
    assert.False(t, ok)
}

func TestHistoryRestoreDefaultsMissingProviderToClaude(t *testing.T) {
    // Persist legacy metadata JSON without provider, restore, assert Provider == "claude".
}
```

- [ ] **Step 2: Verify focused tests fail**

Run: `go test ./pkg/provider ./pkg/protocol ./pkg/session ./pkg/engine -run 'Provider|Legacy|Metadata' -v`

Expected: FAIL on missing provider types/fields.

- [ ] **Step 3: Add provider descriptors and metadata fields**

```go
type PermissionOption struct {
    ID string `json:"id"`
    Label string `json:"label"`
    Description string `json:"description"`
    Dangerous bool `json:"dangerous,omitempty"`
    Mutable bool `json:"mutable"`
}
type Descriptor struct {
    ID string `json:"id"`
    Name string `json:"name"`
    Available bool `json:"available"`
    UnavailableReason string `json:"unavailableReason,omitempty"`
    Permissions []PermissionOption `json:"permissions"`
}
type SessionConfig struct { Cwd, SessionID, Permission, Model string; Resume bool; AddDirs, AllowedTools []string }
type Adapter interface { Descriptor() Descriptor; NewProcess(SessionConfig) Process }
```

Extend `ControlMsg`, `SessionInfo`, and `SessionCreatedMsg` with `provider`, optional `model`, `permissionMode`, and `requestId`. Add `provider_list` and `set_permission_mode` protocol constants. Extend session metadata and make legacy restore set `Provider = "claude"`.

- [ ] **Step 4: Run package tests**

Run: `go test ./pkg/provider ./pkg/protocol ./pkg/session ./pkg/engine`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/provider pkg/protocol pkg/session pkg/engine/history.go pkg/engine/persistence_test.go
git commit -m "feat: add provider-neutral session metadata"
```

### Task 3: Idempotent Provider-Based Session Creation

**Files:**
- Modify: `pkg/engine/engine.go`
- Modify: `pkg/engine/wsserver.go`
- Modify: `pkg/engine/wsserver_test.go`
- Modify: `pkg/engine/reliability_test.go`

**Interfaces:**
- Consumes: `provider.Registry`, `session.Config`, and protocol provider/request fields from Task 2.
- Produces: `Engine.SetProviderRegistry(*provider.Registry)` for tests and future providers.
- Produces: in-memory `createRequests map[string]createResult` keyed by `deviceID + "\x00" + requestID`.

- [ ] **Step 1: Write WebSocket behavior tests**

```go
func TestCreateSessionDefaultsProviderAndEchoesRequestID(t *testing.T) {
    // Send create_session without provider and assert provider=claude and requestId echoed.
}

func TestCreateSessionRequestIDIsIdempotent(t *testing.T) {
    // Send the same create_session twice and assert the same sessionId and one process start.
}

func TestCreateSessionRejectsUnknownProvider(t *testing.T) {
    // provider=codex returns PROVIDER_NOT_AVAILABLE and creates no session.
}
```

- [ ] **Step 2: Verify tests fail**

Run: `go test ./pkg/engine -run 'CreateSession.*Provider|RequestID|UnknownProvider' -v`

Expected: FAIL because creation is still Claude-specific and non-idempotent.

- [ ] **Step 3: Replace `ClaudeFactory` lookup with the registry**

Normalize provider ID before lookup, validate the selected permission against the descriptor, create `session.Config`, persist it, and call `adapter.NewProcess`. If a non-empty request ID already has a result for the same device, return the stored `session_created`; if its parameters differ, return `REQUEST_ID_CONFLICT`. Include descriptor data in `provider_list` responses.

```go
adapter, ok := e.providers.Get(provider.NormalizeID(msg.Provider))
if !ok || !adapter.Descriptor().Available { return "", ErrProviderNotAvailable }
key := cl.deviceID + "\x00" + msg.RequestID
if prior, ok := e.createRequests[key]; ok { return prior.SessionID, cl.writeJSON(prior.Message) }
```

- [ ] **Step 4: Run engine and race tests**

Run: `go test ./pkg/engine && go test -race ./pkg/engine`

Expected: PASS with no races.

- [ ] **Step 5: Commit**

```bash
git add pkg/engine
git commit -m "feat: create sessions through provider registry"
```

### Task 4: Safe Permission Changes for Existing Claude Sessions

**Files:**
- Modify: `pkg/session/session.go`
- Modify: `pkg/engine/history.go`
- Modify: `pkg/engine/engine.go`
- Modify: `pkg/engine/wsserver.go`
- Modify: `pkg/engine/output.go`
- Modify: `pkg/engine/wsserver_test.go`

**Interfaces:**
- Produces: `session.Session.SetPermission(string)` and `historyStore.UpdateSession(*session.Session) error`.
- Produces: `permission_changed` payload with `sessionId`, `permissionMode`, and `pending`.
- Consumes: engine `busy` state; Claude fallback stops only while idle and resumes the same CLI session ID.

- [ ] **Step 1: Add idle, busy, rollback, and ownership tests**

```go
func TestPermissionChangeRestartsIdleClaudeSessionAndPersists(t *testing.T) {
    engine, client, factory := newPermissionTestEngine(t)
    sessionID := createOwnedSession(t, engine, client, "default")
    writeControl(t, client, protocol.ControlMsg{Action: protocol.ActionSetPermissionMode, SessionID: sessionID, PermissionMode: "plan"})
    assertPermissionChanged(t, client, sessionID, "plan", false)
    assert.Equal(t, []string{"default", "plan"}, factory.StartedPermissions())
    assert.Equal(t, "plan", restoreSessionMetadata(t, engine.cfg.DataDir, sessionID).Permission)
}
func TestPermissionChangeWhileBusyIsAppliedAfterDone(t *testing.T) {
    engine, client, factory := newPermissionTestEngine(t)
    sessionID := createBusyOwnedSession(t, engine, client)
    writeControl(t, client, protocol.ControlMsg{Action: protocol.ActionSetPermissionMode, SessionID: sessionID, PermissionMode: "plan"})
    assertPermissionChanged(t, client, sessionID, "default", true)
    factory.EmitDone(sessionID)
    assertPermissionChanged(t, client, sessionID, "plan", false)
}
func TestPermissionChangeFailureKeepsPreviousMode(t *testing.T) {
    engine, client, factory := newPermissionTestEngine(t)
    sessionID := createOwnedSession(t, engine, client, "default")
    factory.FailNextStart(errors.New("resume failed"))
    writeControl(t, client, protocol.ControlMsg{Action: protocol.ActionSetPermissionMode, SessionID: sessionID, PermissionMode: "plan"})
    assertProtocolError(t, client, "ENGINE_ERROR")
    assert.Equal(t, "default", mustSession(t, engine, sessionID).Permission)
}
func TestPermissionChangeRequiresOwner(t *testing.T) {
    engine, _, _ := newPermissionTestEngine(t)
    owner, guest := connectTwoClients(t, engine)
    sessionID := createOwnedSession(t, engine, owner, "default")
    writeControl(t, guest, protocol.ControlMsg{Action: protocol.ActionSetPermissionMode, SessionID: sessionID, PermissionMode: "plan"})
    assertProtocolError(t, guest, protocol.CodeSessionNotOwner)
}
```

- [ ] **Step 2: Verify the tests fail**

Run: `go test ./pkg/engine -run PermissionChange -v`

Expected: FAIL because `set_permission_mode` is unsupported.

- [ ] **Step 3: Implement pending-at-busy-boundary semantics**

Add `pendingPermission map[string]string`. An idle change creates and starts the replacement process with `Resume: true` before swapping it into `e.procs`; only then stop the old process, set/persist permission, and broadcast success. A busy change records pending and broadcasts `pending:true`; the existing done-output path invokes the same idle helper before dequeuing later prompts. On any failure retain the old process and old metadata.

- [ ] **Step 4: Run permission and regression tests**

Run: `go test ./pkg/engine ./pkg/session && go test -race ./pkg/engine`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/session pkg/engine
git commit -m "feat: support safe session permission changes"
```

### Task 5: Native Finder Folder Picker and Local Project Authorization

**Files:**
- Modify: `pkg/desktop/native_darwin.h`
- Modify: `pkg/desktop/native_darwin.m`
- Modify: `pkg/desktop/native_darwin.go`
- Modify: `pkg/desktop/native_stub.go`
- Modify: `pkg/desktop/native.go`
- Modify: `pkg/desktop/server.go`
- Modify: `pkg/desktop/server_test.go`
- Modify: `cmd/mac-app/main.go`

**Interfaces:**
- Produces: JS binding `window.codeAfarNative.chooseDirectory(): Promise<string>`; cancellation resolves to `""`.
- Produces: loopback-only `POST /desktop/projects` accepting `{path}` and returning `{name,path,permission}`.
- Consumes: existing `projectStore.Add` through a `HandlerOptions.AddProject(path string) (protocol.ProjectInfo, error)` callback.

- [ ] **Step 1: Write local endpoint tests**

```go
func TestDesktopProjectEndpointRejectsNonLoopbackAndFiles(t *testing.T) {
    handler := NewHandler(HandlerOptions{AddProject: func(string) (protocol.ProjectInfo, error) { t.Fatal("must not authorize"); return protocol.ProjectInfo{}, nil }})
    request := httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(`{"path":"/tmp/file"}`))
    request.RemoteAddr = "100.64.0.2:1234"
    response := httptest.NewRecorder()
    handler.ServeHTTP(response, request)
    assert.Equal(t, http.StatusForbidden, response.Code)
}
func TestDesktopProjectEndpointAddsExistingDirectory(t *testing.T) {
    directory := t.TempDir()
    handler := NewHandler(HandlerOptions{AddProject: func(path string) (protocol.ProjectInfo, error) {
        assert.Equal(t, directory, path)
        return protocol.ProjectInfo{Name: filepath.Base(path), Path: path, Permission: "default"}, nil
    }})
    response := httptest.NewRecorder()
    request := httptest.NewRequest(http.MethodPost, "/desktop/projects", strings.NewReader(fmt.Sprintf(`{"path":%q}`, directory)))
    request.RemoteAddr = "127.0.0.1:1234"
    handler.ServeHTTP(response, request)
    assert.Equal(t, http.StatusCreated, response.Code)
}
```

- [ ] **Step 2: Verify endpoint tests fail**

Run: `go test ./pkg/desktop -run DesktopProjectEndpoint -v`

Expected: FAIL with 404.

- [ ] **Step 3: Add the Cocoa picker and WebView binding**

```objc
char *caChooseDirectory(void) {
  NSOpenPanel *panel = [NSOpenPanel openPanel];
  panel.canChooseDirectories = YES;
  panel.canChooseFiles = NO;
  panel.allowsMultipleSelection = NO;
  if ([panel runModal] != NSModalResponseOK) return strdup("");
  return strdup(panel.URL.path.UTF8String);
}
```

Bind a Go wrapper named `codeAfarChooseDirectory`, expose it as `window.codeAfarNative.chooseDirectory`, validate the returned path again in Go, and authorize it through the desktop-only endpoint. Free the C string after conversion.

- [ ] **Step 4: Run desktop tests and compile the native app**

Run: `go test ./pkg/desktop && go build ./cmd/mac-app`

Expected: PASS and successful macOS CGO link.

- [ ] **Step 5: Commit**

```bash
git add pkg/desktop cmd/mac-app/main.go
git commit -m "feat(mac): choose and authorize Finder projects"
```

### Task 6: Draft-First Web Session State Machine

**Files:**
- Modify: `web/chat/index.html`
- Modify: `web/chat/chat.js`
- Modify: `web/chat/core.css`
- Modify: `web/chat/desktop.css`
- Modify: `web/chat/mobile.css`
- Modify: `web/design_regression_test.go`
- Create: `web/chat_state_test.go`

**Interfaces:**
- Consumes: `provider_list`, extended `session_list`, `session_created`, `permission_changed`, folder picker, and project endpoint.
- Produces: `window.codeAfar` with `setPrompt(text)`, `setVoiceState(state, message)`, and `showAdmin()`; `window.claudePhone` aliases the same frozen object for one compatibility release.
- Produces: client states `draft`, `creating`, `ready`, `failed` and a stable request ID retained across retries.

- [ ] **Step 1: Add DOM/JavaScript contract tests**

```go
func TestNewSessionUsesComposerContextBar(t *testing.T) {
    // Assert top header lacks create-project/create-permission and draft context controls exist.
}
func TestChatScriptCreatesThenSendsPendingFirstPrompt(t *testing.T) {
    // Assert requestId/provider fields, pending prompt, session_created gate, and one send.
}
func TestChatScriptExposesCodeAfarBridgeAlias(t *testing.T) {
    source := readAsset(t, "chat/chat.js")
    assert.Contains(t, source, "window.codeAfar = Object.freeze")
    assert.Contains(t, source, "window.claudePhone = window.codeAfar")
}
```

- [ ] **Step 2: Verify contract tests fail**

Run: `go test ./web -run 'NewSession|CreatesThenSends|CodeAfarBridge' -v`

Expected: FAIL on the old header controls and immediate create behavior.

- [ ] **Step 3: Build the draft page and deterministic title helper**

```js
function sessionNameFor(promptText, cwd) {
  const normalized = promptText.trim().replace(/\s+/g, " ");
  return Array.from(normalized).slice(0, 32).join("") || `${basename(cwd)} · ${new Date().toLocaleTimeString([], {hour: "2-digit", minute: "2-digit"})}`;
}

function beginDraft() {
  state.sessionId = "";
  state.sessionReady = false;
  state.draft = { status: "draft", requestId: crypto.randomUUID(), firstPrompt: "" };
  renderDraft();
}
```

On submit in draft state, validate connected/provider/project/permission/prompt, retain `firstPrompt`, send one `create_session`, and set `creating`. On matching `session_created`, select the session, append/send the retained text once, clear it, and set `ready`. On error or reconnect, retain text/request ID and render a retry action. Return submits; Shift-Return inserts a newline. New Session and Command-N only call `beginDraft()`.

Render folder/provider/permission controls below the textarea. Hide provider/model selectors when there is only one choice. Existing sessions show folder/provider summary and editable permission; dangerous permission requires `window.confirm`.

- [ ] **Step 4: Run web tests and syntax checks**

Run: `go test ./web && node --check web/chat/chat.js && node --check web/admin/admin.js`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web
git commit -m "feat(web): create sessions from a composer draft"
```

### Task 7: Rename User-Visible Builds and Preserve the Legacy JS Bridge

**Files:**
- Modify: `pkg/desktop/native_darwin.go`
- Modify: `pkg/desktop/autostart.go`
- Modify: `pkg/desktop/autostart_darwin.go`
- Modify: `scripts/build-mac-app.sh`
- Modify: `scripts/test-mac-reopen.sh`
- Modify: `scripts/package-release.sh`
- Modify: `scripts/Info.plist`
- Modify: `Makefile`
- Modify: `android/app/src/main/AndroidManifest.xml`
- Modify: `android/app/src/main/java/com/claudephone/MainActivity.kt`
- Modify: `android/app/src/main/java/com/claudephone/IPNServiceImpl.kt`
- Modify: `ios/ClaudePhone/Info.plist`
- Modify: `ios/ClaudePhoneTunnel/Info.plist`
- Modify: `ios/ClaudePhone/Views/PairingView.swift`
- Modify: `ios/ClaudePhone/Views/SessionListView.swift`
- Create: `scripts/test-brand-contract.sh`

**Interfaces:**
- Consumes: product constants from Task 1.
- Produces: `build/CodeAfar.app/Contents/MacOS/codeafar` and `build/release/codeafar-*`.
- Preserves: internal Go module path and existing mobile package identifiers.

- [ ] **Step 1: Add a brand artifact contract test**

```bash
#!/usr/bin/env bash
set -euo pipefail
rg -n "Claude Phone" web/chat/index.html scripts/Info.plist ios/ClaudePhone/Info.plist android/app/src/main/AndroidManifest.xml && exit 1 || true
test -x "build/CodeAfar.app/Contents/MacOS/codeafar"
test "$(plutil -extract CFBundleDisplayName raw 'build/CodeAfar.app/Contents/Info.plist')" = "CodeAfar"
```

- [ ] **Step 2: Verify the contract fails against the old artifact**

Run: `bash scripts/test-brand-contract.sh`

Expected: FAIL because user-visible files and app paths still say Claude Phone.

- [ ] **Step 3: Rename display strings, executable paths, launch-agent label, and release files**

Set bundle ID to `com.codeafar.mac`, executable to `codeafar`, app path to `build/CodeAfar.app`, menu title to `CA`, and displayed app names to `CodeAfar`. Retain internal Android namespace and iOS target names to avoid needless migration. Make the installer/launch-agent migration stop old `claude-phone`, install the new app, verify it launches, then remove the old app and old launch-agent plist.

- [ ] **Step 4: Build and verify brand artifacts**

Run: `make mac-app verify-mac-app && bash scripts/test-brand-contract.sh && ./scripts/validate-ios-project.sh`

Expected: PASS and `build/CodeAfar.app` exists.

- [ ] **Step 5: Commit**

```bash
git add pkg/desktop scripts Makefile android ios web/chat/index.html
git commit -m "feat: rename product artifacts to CodeAfar"
```

### Task 8: Mac Installation and End-to-End Acceptance

**Files:**
- Modify: `README.md`
- Modify: `docs/INSTALL-MAC.md`
- Modify: `docs/TESTING.md`
- Modify: `scripts/install-mac-app.sh`
- Create: `scripts/test-codeafar-session.sh`

**Interfaces:**
- Produces: `/Applications/CodeAfar.app` and a repeatable smoke test that checks status, draft behavior, session idempotency, and a real Claude response.

- [ ] **Step 1: Add the smoke-test assertions before installation changes**

The script must fail unless the app bundle is signed, the executable starts, `/desktop/status` reports ready, the page contains `CodeAfar`, two creates with one request ID return one session ID, and a text prompt produces at least one `token` or `done` event within the bounded timeout.

- [ ] **Step 2: Run the smoke test to capture the expected pre-install failure**

Run: `bash scripts/test-codeafar-session.sh`

Expected: FAIL because `/Applications/CodeAfar.app` is not installed yet.

- [ ] **Step 3: Complete install/upgrade flow and documentation**

Document that New Session opens a draft, Finder authorizes folders, `每次询问` is the safe default, and provider/folder are fixed after creation. Install via a staged temporary app path, launch and poll readiness, then remove `/Applications/Claude Phone.app` only after success.

- [ ] **Step 4: Run the complete Mac acceptance suite**

Run: `make verify && make mac-release && ./scripts/install-mac-app.sh && ./scripts/test-mac-reopen.sh && ./scripts/test-codeafar-session.sh`

Expected: all commands PASS; a real Claude reply is observed; `/Applications/CodeAfar.app` opens; old data is visible after migration.

- [ ] **Step 5: Commit**

```bash
git add README.md docs scripts
git commit -m "docs: add CodeAfar installation and acceptance flow"
```

## Final Verification

- [ ] Run `go test ./...` and expect PASS.
- [ ] Run `go test -race ./pkg/engine` and expect PASS.
- [ ] Run `node --check web/chat/chat.js && node --check web/admin/admin.js` and expect no output.
- [ ] Run `make verify` and expect PASS.
- [ ] Run `make mac-release` and expect a `codeafar-macos-*.zip` plus valid `SHA256SUMS`.
- [ ] Launch `/Applications/CodeAfar.app`, choose a Finder folder, send `你好`, observe a real reply, copy it, and verify Return sends while Shift-Return inserts a newline.
