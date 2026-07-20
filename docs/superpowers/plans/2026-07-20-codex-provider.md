# CodeAfar Codex Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a production-ready Codex provider that reuses the locally authenticated Codex CLI and behaves like the existing Claude provider in CodeAfar sessions.

**Architecture:** Keep CodeAfar's provider-neutral engine and WebSocket protocol. Add a one-process-per-turn Codex driver around stable `codex exec --json`, persist the returned Codex thread ID separately from the CodeAfar session ID, and translate Codex JSONL events through an optional provider translator. Discover Claude and Codex independently so either provider can keep the Mac app operational.

**Tech Stack:** Go 1.22+, Codex CLI JSONL, Gorilla WebSocket, vanilla JavaScript, macOS WebView/Cocoa packaging, Go and Node tests.

## Global Constraints

- Reuse the local Codex CLI login and user configuration; never store an OpenAI API key.
- Preserve the existing new-session Draft UI and provider-neutral WebSocket protocol.
- Existing sessions remain bound to their original provider.
- Empty model means the Codex CLI configured default; do not add model selection in this delivery.
- Use stable `codex exec --json`; do not use experimental app-server or add an SDK sidecar.
- Codex permissions are `readOnly`, `workspaceWrite`, and `fullAccess`; do not reuse Claude permission IDs.
- Missing Codex must not prevent Claude sessions, and missing Claude must not prevent Codex sessions.
- Preserve legacy session metadata that has no `providerSessionId`.
- Do not expose credentials or unbounded stderr in client errors.

---

### Task 1: Persist Provider Session Identity

**Files:**
- Modify: `pkg/provider/provider.go`
- Modify: `pkg/session/session.go`
- Modify: `pkg/session/session_test.go`
- Modify: `pkg/engine/history.go`
- Modify: `pkg/engine/persistence_test.go`
- Modify: `pkg/engine/wsserver.go`
- Modify: `pkg/engine/permission.go`

**Interfaces:**
- Produces: `SessionConfig.ProviderSessionID string`.
- Produces: optional `provider.SessionIdentity` with `ProviderSessionID() string`.
- Produces: `Session.SetProviderSessionID(string) bool` and `Session.ProviderSessionIdentity() string`.
- Persists: optional JSON field `providerSessionId` in `meta.json`.

- [ ] **Step 1: Write failing identity and persistence tests**

```go
func TestSessionProviderIdentityUpdatesOnlyOnChange(t *testing.T) {
    s := NewSession("local", "name", ".", "owner")
    if !s.SetProviderSessionID("codex-thread") || s.SetProviderSessionID("codex-thread") {
        t.Fatal("provider identity change tracking is incorrect")
    }
    if got := s.ProviderSessionIdentity(); got != "codex-thread" { t.Fatalf("got %q", got) }
}

func TestHistoryRoundTripsProviderSessionID(t *testing.T) {
    store := newHistoryStore(t.TempDir())
    s := session.NewSession("local", "codex", ".", "owner")
    s.Provider = provider.CodexID
    s.SetProviderSessionID("thread-123")
    if err := store.CreateSession(s); err != nil { t.Fatal(err) }
    restored, err := store.Restore()
    if err != nil || restored[0].ProviderSessionIdentity() != "thread-123" { t.Fatalf("restored=%+v err=%v", restored, err) }
}
```

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./pkg/session ./pkg/engine -run 'ProviderIdentity|ProviderSessionID' -count=1`

Expected: compilation fails because the provider identity API does not exist.

- [ ] **Step 3: Implement the identity fields and atomic metadata update**

```go
type SessionConfig struct {
    Cwd, SessionID, ProviderSessionID, Permission, Model string
    Resume bool
    AddDirs, AllowedTools []string
}

type SessionIdentity interface { ProviderSessionID() string }

func (s *Session) SetProviderSessionID(id string) bool {
    s.mu.Lock()
    defer s.mu.Unlock()
    if id == "" || s.providerSessionID == id { return false }
    s.providerSessionID = id
    return true
}
```

Add `ProviderSessionID string \`json:"providerSessionId,omitempty"\`` to `sessionMeta`; save and restore it. Pass it to every adapter process created for new, resumed, or permission-restarted sessions. In `handleProcOutput`, detect `provider.SessionIdentity`, persist a newly observed identity before translating the event, and broadcast `ENGINE_ERROR` if persistence fails.

- [ ] **Step 4: Run focused and race tests**

Run: `go test ./pkg/session ./pkg/engine -count=1 && go test -race ./pkg/engine -count=1`

Expected: PASS with no races.

- [ ] **Step 5: Commit**

```bash
git add pkg/provider/provider.go pkg/session pkg/engine/history.go pkg/engine/persistence_test.go pkg/engine/wsserver.go pkg/engine/permission.go
git commit -m "feat: persist provider session identity"
```

### Task 2: Implement the Codex CLI Process Driver

**Files:**
- Create: `pkg/session/codex.go`
- Create: `pkg/session/codex_test.go`
- Create: `testdata/fake-codex.sh`

**Interfaces:**
- Produces: `CodexConfig{Bin, Cwd, ProviderSessionID, Permission, Model string; AddDirs []string}`.
- Produces: `NewCodexProc(CodexConfig) *CodexProc` implementing `provider.Process` and `provider.SessionIdentity` structurally.
- Emits raw Codex JSONL to `OnOutput`; captures `thread.started` before invoking the callback.

- [ ] **Step 1: Write failing command construction tests**

```go
func TestCodexProcBuildsNewAndResumeCommands(t *testing.T) {
    fresh := NewCodexProc(CodexConfig{Bin:"codex", Cwd:"/repo", Permission:"workspaceWrite", Model:"gpt-test"})
    if got := fresh.buildArgs("fix it"); !slices.Contains(got, "workspace-write") || !slices.Contains(got, "--json") { t.Fatalf("args=%v", got) }
    resumed := NewCodexProc(CodexConfig{Bin:"codex", Cwd:"/repo", ProviderSessionID:"thread-1", Permission:"readOnly"})
    got := resumed.buildArgs("continue")
    if !containsSequence(got, "exec", "resume") || !slices.Contains(got, "thread-1") { t.Fatalf("args=%v", got) }
}
```

Add tests that `fullAccess` maps to `danger-full-access`, unknown permissions fail, a second concurrent `Send` fails, and `Stop` kills the active child without emitting a synthetic provider error.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./pkg/session -run CodexProc -count=1`

Expected: compilation fails because `CodexProc` does not exist.

- [ ] **Step 3: Implement a serialized one-process-per-turn driver**

```go
func (p *CodexProc) buildArgs(prompt string) ([]string, error) {
    sandbox, ok := map[string]string{"readOnly":"read-only", "workspaceWrite":"workspace-write", "fullAccess":"danger-full-access"}[p.cfg.Permission]
    if !ok { return nil, fmt.Errorf("unsupported Codex permission %q", p.cfg.Permission) }
    args := []string{"-C", p.cfg.Cwd, "-s", sandbox, "-a", "never"}
    if p.cfg.Model != "" { args = append(args, "-m", p.cfg.Model) }
    for _, dir := range p.cfg.AddDirs { args = append(args, "--add-dir", dir) }
    args = append(args, "exec")
    if p.ProviderSessionID() != "" { args = append(args, "resume") }
    args = append(args, "--json", "--color", "never", "--skip-git-repo-check")
    if id := p.ProviderSessionID(); id != "" { args = append(args, id) }
    return append(args, prompt), nil
}
```

Scan stdout with a 4 MiB limit and stderr with a bounded 16 KiB accumulator. Hold `turn.completed` or `turn.failed` until `cmd.Wait()` has returned and the active command has been cleared, then emit the terminal event so the engine queue can safely start the next turn. On abnormal exit without a terminal event, emit a normalized `CODEX_ERROR` capped at 2 KiB.

- [ ] **Step 4: Run process and race tests**

Run: `go test ./pkg/session -run CodexProc -count=1 && go test -race ./pkg/session -run CodexProc -count=1`

Expected: PASS; fake CLI proves thread capture, terminal ordering, stop, and resume arguments.

- [ ] **Step 5: Commit**

```bash
git add pkg/session/codex.go pkg/session/codex_test.go testdata/fake-codex.sh
git commit -m "feat: drive Codex CLI sessions"
```

### Task 3: Translate Codex JSONL Into the Shared Protocol

**Files:**
- Modify: `pkg/provider/provider.go`
- Create: `pkg/provider/codex.go`
- Create: `pkg/provider/codex_translate.go`
- Create: `pkg/provider/codex_test.go`
- Modify: `pkg/engine/queue.go`
- Modify: `pkg/engine/queue_test.go`
- Modify: `web/chat/tool-format.js`
- Modify: `web/chat/tool-format.test.js`

**Interfaces:**
- Produces: `provider.OutputTranslator` with `TranslateOutput([]byte) [][]byte`.
- Produces: `NewCodexAdapter(bin string, available bool, reason string) Adapter`.
- Produces: descriptor permission IDs `readOnly`, `workspaceWrite`, `fullAccess`.

- [ ] **Step 1: Write failing event translation tests**

```go
func TestCodexTranslatorMapsAgentCommandAndDone(t *testing.T) {
    adapter := NewCodexAdapter("codex", true, "")
    tr := adapter.(OutputTranslator)
    assertProtocol(t, tr.TranslateOutput([]byte(`{"type":"item.completed","item":{"type":"agent_message","text":"hello"}}`)), `{"type":"token","content":"hello"}`)
    assertTool(t, tr.TranslateOutput([]byte(`{"type":"item.started","item":{"id":"1","type":"command_execution","command":"git status"}}`)), "Bash", `{"command":"git status"}`)
    assertProtocol(t, tr.TranslateOutput([]byte(`{"type":"turn.completed"}`)), `{"type":"done"}`)
}
```

Add cases for `file_change`, MCP calls, web search, `turn.failed`, top-level errors, internal warning items, malformed JSON, and unknown items. Assert that completed command items do not duplicate started tool cards.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./pkg/provider ./pkg/engine -run 'Codex|ProviderTranslator' -count=1`

Expected: compilation fails because the Codex adapter and translator are missing.

- [ ] **Step 3: Implement optional provider translation**

```go
type OutputTranslator interface { TranslateOutput(payload []byte) [][]byte }

func (e *Engine) translateOutput(sess *session.Session, payload []byte) [][]byte {
    adapter, ok := e.providers.Get(sess.Provider)
    if ok {
        if translator, ok := adapter.(provider.OutputTranslator); ok {
            return translator.TranslateOutput(payload)
        }
    }
    return translateClaudeOutput(payload)
}
```

Codex translation must marshal shared `TokenMsg`, `ToolUseMsg`, `DoneMsg`, and `NewError("CODEX_ERROR", message)`. Update the JS formatter with labels for Codex file deletion, MCP, and web-search cards while retaining its prototype-safe fallback.

- [ ] **Step 4: Run provider, engine, and Node tests**

Run: `go test ./pkg/provider ./pkg/engine ./web -count=1 && node --test web/chat/tool-format.test.js`

Expected: PASS with no duplicate tool messages.

- [ ] **Step 5: Commit**

```bash
git add pkg/provider pkg/engine/queue.go pkg/engine/queue_test.go web/chat/tool-format.js web/chat/tool-format.test.js
git commit -m "feat: translate Codex execution events"
```

### Task 4: Discover and Register Both Local Providers

**Files:**
- Modify: `pkg/engine/config.go`
- Modify: `pkg/engine/engine.go`
- Modify: `pkg/engine/claude_version.go`
- Modify: `pkg/provider/claude.go`
- Modify: `pkg/provider/provider.go`
- Modify: `pkg/provider/provider_test.go`
- Modify: `pkg/desktop/claude.go`
- Modify: `pkg/desktop/claude_test.go`
- Modify: `pkg/desktop/server.go`
- Modify: `pkg/desktop/server_test.go`
- Modify: `cmd/mac-app/main.go`
- Modify: `cmd/mac-app/application.go`
- Modify: `cmd/mac-app/application_test.go`

**Interfaces:**
- Adds: `Config.CodexBin`, `Config.CodexVersion`, and CLI flag `--codex-bin`.
- Adds: `desktop.ResolveCodexBinary` and general `engine.DetectCLIVersion(bin, product string)`.
- Extends: `AppStatus` with Codex binary/version without removing Claude fields.

- [ ] **Step 1: Write failing discovery and application readiness tests**

```go
func TestApplicationStartsWhenOnlyCodexIsAvailable(t *testing.T) {
    deps := fakeDependencies(claudeError, codexPath)
    app := newApplication(context.Background(), testConfig(), deps)
    if err := app.Start(); err != nil { t.Fatal(err) }
    if !app.Status().Ready || app.Status().CodexBin != codexPath { t.Fatalf("status=%+v", app.Status()) }
}

func TestDefaultRegistryContainsClaudeAndCodexDescriptors(t *testing.T) {
    e := New(Config{DataDir:t.TempDir(), ClaudeBin:"claude", CodexBin:"codex"})
    if _, ok := e.providers.Get(CodexID); !ok { t.Fatal("Codex missing") }
}
```

Add a case where neither provider exists and assert the app reports both actionable failures.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./pkg/provider ./pkg/desktop ./cmd/mac-app -run 'Codex|OnlyCodex|Providers' -count=1`

Expected: compilation fails on missing Codex configuration and status fields.

- [ ] **Step 3: Implement independent provider discovery**

Resolve binaries independently. Build the engine registry with both adapters, preserving unavailable descriptors. `Engine.New` must not assume Claude availability. `application.Resume` returns an error only if neither binary is usable; otherwise it starts the engine and exposes per-provider status.

```go
registry := provider.NewRegistry(
    provider.NewClaudeAdapterWithAvailability(cfg.ClaudeBin, cfg.ClaudeAvailable, cfg.ClaudeUnavailableReason),
    provider.NewCodexAdapter(cfg.CodexBin, cfg.CodexAvailable, cfg.CodexUnavailableReason),
)
```

- [ ] **Step 4: Run application and engine regression tests**

Run: `go test ./pkg/provider ./pkg/desktop ./pkg/engine ./cmd/mac-app -count=1`

Expected: PASS for Claude-only, Codex-only, both, and neither configurations.

- [ ] **Step 5: Commit**

```bash
git add pkg/engine pkg/provider pkg/desktop cmd/mac-app
git commit -m "feat(mac): register available coding providers"
```

### Task 5: Complete Provider-Specific Session UX and Integration Tests

**Files:**
- Modify: `web/chat/chat.js`
- Modify: `web/design_regression_test.go`
- Modify: `pkg/engine/wsserver_test.go`
- Create: `pkg/engine/real_codex_e2e_test.go`
- Modify: `README.md`
- Modify: `docs/testing/mac-v1-acceptance-plan.md`

**Interfaces:**
- Draft provider change selects that provider's first/default permission.
- Session summary resolves provider display name from the descriptor instead of hard-coded Claude logic.
- Opt-in test gate: `CODEAFAR_REAL_CODEX=1`.

- [ ] **Step 1: Write failing Web and engine integration tests**

```go
func TestDraftProviderChangeSelectsProviderPermission(t *testing.T) {
    js := mustAsset(t, "chat/chat.js")
    for _, marker := range []string{"providerSelect.addEventListener(\"change\"", "descriptor.permissions[0].id", "providerName("} {
        if !strings.Contains(js, marker) { t.Fatalf("missing %q", marker) }
    }
}
```

Add an engine test creating `provider:"codex"`, asserting `workspaceWrite` is accepted, Claude's `default` is rejected for Codex, and process configs contain the provider thread ID after restore.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./web ./pkg/engine -run 'DraftProvider|CodexSession|ProviderName' -count=1`

Expected: FAIL because provider changes retain Claude's `default` permission and names are hard-coded.

- [ ] **Step 3: Implement provider-driven draft behavior and documentation**

```js
function providerName(id) {
  return state.providers.find(item => item.id === id)?.name || id || "编码引擎";
}
providerSelect.addEventListener("change", () => {
  const descriptor = currentProvider();
  if (state.draft && descriptor?.permissions?.length) state.draft.permissionMode = descriptor.permissions[0].id;
  renderPermissions(state.draft?.permissionMode || "");
});
```

Use `providerName` in existing-session and `session_created` summaries. Document local `codex login`, provider selection, permission meanings, and the non-interactive approval limitation.

- [ ] **Step 4: Add and run a real Codex two-turn E2E**

The opt-in test creates an isolated CodeAfar data directory, creates a Codex read-only session, asks it to run `pwd` and return a marker, captures a complete Bash tool card, sends a second prompt that references the first marker, closes/rebuilds the engine, resumes by persisted provider thread ID, and verifies a third marker.

Run: `CODEAFAR_REAL_CODEX=1 go test ./pkg/engine -run TestRealCodexEndToEnd -count=1 -v`

Expected: PASS using the authenticated local Codex CLI.

- [ ] **Step 5: Commit**

```bash
git add web pkg/engine/real_codex_e2e_test.go pkg/engine/wsserver_test.go README.md docs/testing/mac-v1-acceptance-plan.md
git commit -m "feat: create and resume Codex sessions"
```

### Task 6: Release Verification and Installed-App Acceptance

**Files:**
- Modify only if verification finds a regression.

**Interfaces:**
- Produces an installed `/Applications/CodeAfar.app` with both provider descriptors.

- [ ] **Step 1: Run complete repository verification**

Run: `make verify && go vet ./...`

Expected: all Go tests, race tests, builds, Node checks, Android AAR contract, and iOS validation pass.

- [ ] **Step 2: Build Android shared-UI consumer**

Run: `cd android && JAVA_HOME=/opt/homebrew/opt/openjdk@17/libexec/openjdk.jdk/Contents/Home ANDROID_HOME="$HOME/Library/Android/sdk" ./gradlew --no-daemon :app:testDebugUnitTest :app:assembleDebug`

Expected: `BUILD SUCCESSFUL`.

- [ ] **Step 3: Package, install, sign, and reopen Mac app**

Run: `make mac-release && make install-mac-app && codesign --verify --deep --strict /Applications/CodeAfar.app && ./scripts/test-mac-reopen.sh`

Expected: app installs, launches, verifies, and exposes the menu/window after reopen.

- [ ] **Step 4: Perform installed-app black-box acceptance**

Use the loopback status endpoint and macOS Accessibility to verify the app is Ready, the new-session Draft shows `Claude Code` and `Codex`, selecting Codex changes the permission options, a fresh Codex prompt returns a response, the tool card contains its command, Return sends, and the conversation remains selectable after app restart.

- [ ] **Step 5: Review, commit fixes if any, and push**

Run an independent read-only code review over the complete feature range, fix all Critical and Important findings with TDD, rerun the full verification commands, then:

```bash
git push origin master
git status --short
```

Expected: push succeeds and the working tree is clean.
