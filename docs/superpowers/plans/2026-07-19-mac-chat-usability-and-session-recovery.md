# Mac Chat Usability and Session Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every chat message copyable, make Return send safely, and transparently recover persisted sessions whose Claude CLI transcript is missing.

**Architecture:** The session package detects Claude transcript presence under the active Claude configuration directory. The engine injects that query and only passes `Resume: true` when the transcript exists; output translation preserves Claude's `errors[]`. The web layer uses native text selection, a clipboard action with a WKWebView-safe fallback, and a composition-safe key handler.

**Tech Stack:** Go 1.24, Claude Code CLI 2.1.212, embedded vanilla JavaScript/CSS, WebSocket protocol, Cocoa WebView.

## Global Constraints

- Preserve all existing Claude Phone history; do not delete or migrate user data.
- Use `CLAUDE_CONFIG_DIR` when set and `~/.claude` otherwise.
- Return sends, Shift+Return inserts a newline, and IME composition Return never sends.
- All message roles support native selection and expose a reliable copy button with a no-permission fallback.
- Every production change starts with a failing regression test.

---

### Task 1: Detect whether a Claude transcript can be resumed

**Files:**
- Create: `pkg/session/claude_session.go`
- Create: `pkg/session/claude_session_test.go`

**Interfaces:**
- Consumes: `CLAUDE_CONFIG_DIR`, `os.UserHomeDir`, persisted session UUID and working directory.
- Produces: `func ClaudeSessionExists(cwd, sessionID string) bool`.

- [ ] **Step 1: Write the failing transcript lookup tests**

```go
func TestClaudeSessionExistsMatchesTranscriptCWD(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	id := "4e2858dd-c712-4f0e-9818-c05191acf107"
	dir := filepath.Join(root, "projects", "encoded-project")
	if err := os.MkdirAll(dir, 0o700); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte("{\"type\":\"system\",\"cwd\":\"/work\"}\n"), 0o600); err != nil { t.Fatal(err) }
	if !ClaudeSessionExists("/work", id) { t.Fatal("existing transcript was not found") }
	if ClaudeSessionExists("/other", id) { t.Fatal("transcript from another cwd matched") }
}

func TestClaudeSessionExistsReturnsFalseForMissingOrUnsafeID(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	if ClaudeSessionExists("/work", "missing") { t.Fatal("missing transcript matched") }
	if ClaudeSessionExists("/work", "*") { t.Fatal("glob metacharacter was accepted") }
}
```

- [ ] **Step 2: Run the tests and verify RED**

Run: `go test ./pkg/session -run TestClaudeSessionExists -count=1`

Expected: build failure because `ClaudeSessionExists` is undefined.

- [ ] **Step 3: Implement the lookup**

```go
func ClaudeSessionExists(cwd, sessionID string) bool {
	if sessionID == "" || filepath.Base(sessionID) != sessionID || strings.ContainsAny(sessionID, "*?[") {
		return false
	}
	root := os.Getenv("CLAUDE_CONFIG_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil { return false }
		root = filepath.Join(home, ".claude")
	}
	target, err := filepath.Abs(cwd)
	if err != nil { return false }
	matches, err := filepath.Glob(filepath.Join(root, "projects", "*", sessionID+".jsonl"))
	if err != nil { return false }
	for _, path := range matches {
		if transcriptHasCWD(path, target) { return true }
	}
	return false
}
```

`transcriptHasCWD` opens the JSONL file, scans with a 4 MiB limit, unmarshals only a `cwd` field, resolves it to an absolute clean path, and returns true on an exact match.

```go
func transcriptHasCWD(path, target string) bool {
	f, err := os.Open(path)
	if err != nil { return false }
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var event struct { Cwd string `json:"cwd"` }
		if json.Unmarshal(scanner.Bytes(), &event) != nil || event.Cwd == "" { continue }
		actual, err := filepath.Abs(event.Cwd)
		if err == nil && filepath.Clean(actual) == filepath.Clean(target) { return true }
	}
	return false
}
```

- [ ] **Step 4: Run package tests and verify GREEN**

Run: `go test ./pkg/session -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/session/claude_session.go pkg/session/claude_session_test.go
git commit -m "fix(session): detect resumable Claude transcripts"
```

### Task 2: Recover missing restored sessions and retain Claude error details

**Files:**
- Modify: `pkg/engine/engine.go`
- Modify: `pkg/engine/wsserver.go`
- Modify: `pkg/engine/wsserver_test.go`
- Modify: `pkg/engine/translate.go`
- Modify: `pkg/engine/translate_test.go`

**Interfaces:**
- Consumes: `session.ClaudeSessionExists(cwd, sessionID string) bool`.
- Produces: engine field `sessionExists func(string, string) bool`; `ClaudeConfig.Resume` reflects transcript presence; error translation falls back to `errors[]`.

- [ ] **Step 1: Write failing engine resume tests**

Create a dormant session, replace `e.sessionExists`, capture the `session.ClaudeConfig` passed to `SetClaudeFactory`, call `e.resumeSession`, and assert:

```go
func TestResumeSessionUsesTranscriptPresence(t *testing.T) {
	for _, tt := range []struct { name string; exists, wantResume bool }{
		{name: "missing starts fresh", exists: false, wantResume: false},
		{name: "present resumes", exists: true, wantResume: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			e := New(Config{DataDir: t.TempDir()})
			defer e.Close()
			e.sessionExists = func(cwd, id string) bool { return tt.exists }
			s := session.NewSession("4e2858dd-c712-4f0e-9818-c05191acf107", "Mac 会话", "/work", "owner")
			s.Permission = "acceptEdits"
			s.SetStatus("dormant")
			e.manager.Restore(s)
			var captured session.ClaudeConfig
			e.SetClaudeFactory(func(cfg session.ClaudeConfig) claudeProc {
				captured = cfg
				return &stubClaudeProc{}
			})
			if err := e.resumeSession(s); err != nil { t.Fatal(err) }
			if captured.Resume != tt.wantResume {
				t.Fatalf("Resume=%v, want %v", captured.Resume, tt.wantResume)
			}
		})
}
```

Add the symmetric case where `sessionExists` returns true and require `captured.Resume`.

- [ ] **Step 2: Run the engine test and verify RED**

Run: `go test ./pkg/engine -run TestResumeSession -count=1`

Expected: FAIL because `resumeSession` always sets `Resume: true`.

- [ ] **Step 3: Inject the transcript query and use it**

Initialize in `New`:

```go
sessionExists: session.ClaudeSessionExists,
```

Build resumed process configuration with:

```go
Resume: e.sessionExists(s.Cwd, s.ID),
```

- [ ] **Step 4: Write the failing translator test**

```go
func TestTranslateClaudeErrorFallsBackToErrorsArray(t *testing.T) {
	got := translateClaudeOutput([]byte(`{"type":"result","is_error":true,"result":"","errors":["No conversation found with session ID: abc"]}`))
	if len(got) != 1 { t.Fatalf("translated error = %q", got) }
	var message protocol.ErrorMsg
	if err := json.Unmarshal(got[0], &message); err != nil { t.Fatal(err) }
	if message.Message != "No conversation found with session ID: abc" { t.Fatalf("message=%q", message.Message) }
}
```

- [ ] **Step 5: Run the translator test and verify RED**

Run: `go test ./pkg/engine -run TestTranslateClaudeErrorFallsBackToErrorsArray -count=1`

Expected: FAIL because the translated message is empty.

- [ ] **Step 6: Preserve `errors[]` in translated output**

Add `Errors []string` to the raw result envelope. For an error result, use trimmed `result`; when empty, join non-empty trimmed `errors` with `; `. If both are empty, use `Claude exited with an unspecified error`.

- [ ] **Step 7: Run engine tests and verify GREEN**

Run: `go test ./pkg/engine -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/engine/engine.go pkg/engine/wsserver.go pkg/engine/wsserver_test.go pkg/engine/translate.go pkg/engine/translate_test.go
git commit -m "fix(engine): recover missing Claude sessions"
```

### Task 3: Enable reliable copy and Return-to-send

**Files:**
- Modify: `web/chat/desktop.css`
- Modify: `web/chat/chat.js`
- Modify: `web/design_regression_test.go`

**Interfaces:**
- Consumes: native WebView selection, Clipboard API/fallback copy, textarea keydown events and `composer.requestSubmit()`.
- Produces: selectable `.message` elements, per-message copy actions, and composition-safe Return submission.

- [ ] **Step 1: Write failing web asset tests**

```go
func TestDesktopMessagesAreSelectable(t *testing.T) {
	cssBytes, err := fs.ReadFile(Assets, "chat/desktop.css")
	if err != nil { t.Fatal(err) }
	css := string(cssBytes)
	marker := ".message { max-width: 78%; margin: 0 0 16px; padding: 12px 15px; border-radius: 14px; white-space: pre-wrap; user-select: text; cursor: text; }"
	if !strings.Contains(css, marker) { t.Fatalf("message selection rule missing %q", marker) }
}

func TestDesktopReturnSendsWithoutBreakingIME(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil { t.Fatal(err) }
	js := string(jsBytes)
	marker := `prompt.addEventListener("keydown", event => {
    if (event.isComposing || event.keyCode === 229) return;
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      composer.requestSubmit();
    }
  });`
	if !strings.Contains(js, marker) { t.Fatal("composition-safe Return-to-send handler missing") }
}
```

- [ ] **Step 2: Run web tests and verify RED**

Run: `go test ./web -run 'TestDesktopMessagesAreSelectable|TestDesktopReturnSendsWithoutBreakingIME' -count=1`

Expected: FAIL because full-message selection and Return behavior are absent.

- [ ] **Step 3: Implement message selection**

Change the message rule to include:

```css
.message { user-select: text; cursor: text; }
```

Remove the narrower assistant/tool-only selection rule.

Add an accessible copy button to each rendered message. Use `navigator.clipboard.writeText` when available and fall back to a temporary selected textarea plus `document.execCommand("copy")` when WKWebView rejects the Clipboard API.

- [ ] **Step 4: Implement the key handler**

```js
prompt.addEventListener("keydown", event => {
  if (event.isComposing || event.keyCode === 229) return;
  if (event.key === "Enter" && !event.shiftKey) {
    event.preventDefault();
    composer.requestSubmit();
  }
});
```

- [ ] **Step 5: Run web tests and JavaScript syntax check**

Run: `go test ./web -count=1 && node --check web/chat/chat.js`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add web/chat/desktop.css web/chat/chat.js web/design_regression_test.go
git commit -m "feat(mac): support copy and Return-to-send"
```

### Task 4: Package and run installed-app regression

**Files:**
- Modify only if a new regression is discovered.

**Interfaces:**
- Consumes: final packaged application and the existing persisted failing session `4e2858dd-c712-4f0e-9818-c05191acf107`.
- Produces: installed `/Applications/Claude Phone.app` and pushed commits.

- [ ] **Step 1: Run complete automated gates**

Run: `make verify && go vet ./...`

Expected: exit 0.

- [ ] **Step 2: Build, install and verify signature**

Run: `make mac-release`, install `build/Claude Phone.app` with `ditto`, then run `codesign --verify --deep --strict '/Applications/Claude Phone.app'`.

Expected: all commands exit 0.

- [ ] **Step 3: Browser keyboard and copy regression**

Against an isolated fake-Claude data directory:

1. Create a session.
2. Fill `第一行`, press Shift+Return, type `第二行`, and assert no user message was sent yet.
3. Press Return and assert exactly one user message containing both lines and exactly one `hello world` response.
4. Click the assistant message copy action and assert the system clipboard exactly matches the message content.
5. Assert browser console errors/warnings are empty.

- [ ] **Step 4: Reproduce the user's restored-session path with real Claude**

Quit the old installed process, launch the new app with the existing default data directory, select session `4e2858dd-c712-4f0e-9818-c05191acf107`, send `你好`, and require a non-error assistant response. Assert the Claude child uses `--session-id`, not `--resume`, for this transcript-missing session.

- [ ] **Step 5: Quit and audit cleanup**

Require zero Claude Phone processes, zero app-owned Claude processes, zero `/usr/bin/caffeinate -s`, and no listener on port 9877.

- [ ] **Step 6: Commit any evidence-only test changes and push**

Run: `git status --short`, then `git push origin master` after the worktree is clean and local HEAD contains all fixes.
