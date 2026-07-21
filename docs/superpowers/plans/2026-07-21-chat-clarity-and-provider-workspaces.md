# Chat Clarity and Provider Workspaces Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide internal tool commands from normal CodeAfar conversations and replace the sidebar brand/new-session stack with provider-specific Claude/Codex workspaces.

**Architecture:** Keep `tool_use` in the server protocol and persisted history, but make every chat client ignore it. Add a small, testable Web workspace-state module that owns provider filtering and non-sensitive local preferences; keep DOM transitions in `chat.js`. Mirror the same provider/session model in the native iOS store and UI.

**Tech Stack:** Go 1.22+ embedded-asset contract tests, vanilla JavaScript with Node's built-in test runner, HTML/CSS, Swift 6/SwiftUI/XCTest, macOS WebView packaging.

## Global Constraints

- Existing sessions never change provider and Claude/Codex context is never merged.
- Normal chat and restored history never render tool names, parameters, shell commands, or file paths from `tool_use`.
- The server continues translating, transmitting, and persisting tool events.
- The active provider, visible history, current conversation, permissions, and new-session provider always agree.
- Provider preferences may persist provider IDs and session IDs only; they must not store credentials or chat text.
- No new runtime dependency or framework is introduced.

---

### Task 1: Stop Rendering Internal Tool Activity

**Files:**
- Modify: `web/design_regression_test.go`
- Modify: `web/assets_test.go`
- Modify: `web/chat/chat.js`
- Modify: `web/chat/index.html`
- Modify: `web/chat/desktop.css`
- Delete: `web/chat/tool-format.js`
- Delete: `web/chat/tool-format.test.js`
- Modify: `ios/ClaudePhoneTests/ChatStoreTests.swift`
- Modify: `ios/ClaudePhone/Stores/ChatStore.swift`

**Interfaces:**
- Consumes: existing protocol messages with `type == "tool_use"`.
- Produces: chat histories containing only user, assistant, queue, health, and error presentation; no replacement protocol.

- [ ] **Step 1: Write failing Web and iOS tests**

Replace the old readable-tool-card contract with this Web regression:

```go
func TestInternalToolActivityDoesNotEnterChat(t *testing.T) {
    htmlBytes, _ := fs.ReadFile(Assets, "chat/index.html")
    jsBytes, _ := fs.ReadFile(Assets, "chat/chat.js")
    cssBytes, _ := fs.ReadFile(Assets, "chat/desktop.css")
    combined := string(htmlBytes) + string(jsBytes) + string(cssBytes)
    for _, forbidden := range []string{
        "tool-format.js", "formatToolUse", `append("tool"`,
        `Bash: ["执行命令"`, `.message.tool`,
    } {
        if strings.Contains(combined, forbidden) {
            t.Errorf("internal tool UI remains %q", forbidden)
        }
    }
    if !strings.Contains(string(jsBytes), `case "tool_use":\n        break;`) {
        t.Error("live tool events must be explicitly ignored")
    }
}
```

Remove `chat/tool-format.js` and the `msg.tool` rendering requirement from `TestSharedChatAssetsSupportMobileRemoteConnection`. Add this XCTest:

```swift
func testToolActivityIsIgnoredLiveAndInHistory() async {
    let socket = WebSocketClient()
    let store = ChatStore(socket: socket)
    store.handle(.toolUse(tool: "Bash", input: #"{"command":"pwd"}"#))
    XCTAssertTrue(store.messages.isEmpty)
    store.handle(.history(sessionID: "s", messages: [
        HistoryItem(type: "text", content: "hello", tool: nil, input: nil),
        HistoryItem(type: "tool_use", content: nil, tool: "Read", input: "{}"),
        HistoryItem(type: "token", content: "done", tool: nil, input: nil),
    ]))
    try? await Task.sleep(for: .milliseconds(30))
    XCTAssertEqual(store.messages.map(\.text), ["hello", "done"])
}
```

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./web -run TestInternalToolActivityDoesNotEnterChat -count=1
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild test -quiet -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=macOS,arch=arm64,variant=Designed for [iPad,iPhone]' CODE_SIGNING_ALLOWED=NO -only-testing:ClaudePhoneTests/ChatStoreTests
```

Expected: Web test reports the existing formatter/tool append paths; XCTest reports one or more tool messages instead of an empty/clean history.

- [ ] **Step 3: Implement the minimal presentation change**

In `chat.js`, remove the formatter import, remove the history `tool_use` branch, and make the live branch a no-op:

```javascript
      case "tool_use":
        break;
```

Remove the formatter `<script>` from `index.html`, remove `.message.tool` from `desktop.css`, and delete both formatter files. In `ChatStore.swift`, use:

```swift
        case .toolUse: break
```

and in `replay(_:)` use:

```swift
        case "tool_use": break
```

Do not flush tokens or finish the assistant when ignoring a tool event, so text before and after an internal operation stays in the same assistant response.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the two commands from Step 2, then:

```bash
go test ./web ./pkg/product
node --check web/chat/chat.js
```

Expected: all commands exit 0 with no `执行命令`, `formatToolUse`, or tool bubble contract remaining.

- [ ] **Step 5: Commit**

```bash
git add web ios/ClaudePhone/Stores/ChatStore.swift ios/ClaudePhoneTests/ChatStoreTests.swift
git commit -m "fix: keep internal tools out of chat"
```

---

### Task 2: Add a Tested Provider Workspace State Module

**Files:**
- Create: `web/chat/provider-workspace.js`
- Create: `web/chat/provider-workspace.test.js`
- Modify: `web/assets_test.go`
- Modify: `web/chat/index.html`

**Interfaces:**
- Produces: `CodeAfarProviderWorkspace.load(storage)`, `save(storage, value)`, `availableProvider(providers, preferred)`, `sessionsForProvider(sessions, providerID)`, and `rememberedSession(sessions, providerID, lastSessions)`.
- Consumes: Web Storage-compatible objects with `getItem(key)` and `setItem(key, value)`.

- [ ] **Step 1: Write the failing state-module tests**

Create `provider-workspace.test.js`:

```javascript
const test = require("node:test");
const assert = require("node:assert/strict");
const workspace = require("./provider-workspace.js");

test("filters and restores sessions independently per provider", () => {
  const sessions = [
    { sessionId: "c1", provider: "claude" },
    { sessionId: "x1", provider: "codex" },
  ];
  assert.deepEqual(workspace.sessionsForProvider(sessions, "codex"), [sessions[1]]);
  assert.equal(workspace.rememberedSession(sessions, "claude", { claude: "c1", codex: "x1" }), sessions[0]);
  assert.equal(workspace.rememberedSession(sessions, "codex", { codex: "missing" }), null);
});

test("persists only provider and session identifiers", () => {
  const values = new Map();
  const storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
  workspace.save(storage, { activeProvider: "codex", lastSessions: { claude: "c1", codex: "x1" } });
  assert.deepEqual(workspace.load(storage), { activeProvider: "codex", lastSessions: { claude: "c1", codex: "x1" } });
  assert.doesNotMatch(values.values().next().value, /token|content|message/i);
});

test("falls back to the first available provider", () => {
  const providers = [
    { id: "claude", available: false },
    { id: "codex", available: true },
  ];
  assert.equal(workspace.availableProvider(providers, "claude"), "codex");
  assert.equal(workspace.availableProvider([], "claude"), "");
});
```

- [ ] **Step 2: Run the Node test and verify RED**

Run: `node --test web/chat/provider-workspace.test.js`

Expected: FAIL because `provider-workspace.js` does not exist.

- [ ] **Step 3: Implement the focused module**

Create an IIFE module using the exact API below:

```javascript
(function installProviderWorkspace(root) {
  const storageKey = "codeafar.providerWorkspace.v1";
  const clean = value => ({
    activeProvider: typeof value?.activeProvider === "string" ? value.activeProvider : "",
    lastSessions: Object.fromEntries(Object.entries(value?.lastSessions || {}).filter(
      ([provider, session]) => typeof provider === "string" && typeof session === "string"
    )),
  });
  function load(storage) {
    try { return clean(JSON.parse(storage?.getItem(storageKey) || "{}")); }
    catch { return clean({}); }
  }
  function save(storage, value) {
    try { storage?.setItem(storageKey, JSON.stringify(clean(value))); } catch {}
  }
  function availableProvider(providers, preferred) {
    return providers.find(item => item.id === preferred && item.available)?.id
      || providers.find(item => item.available)?.id || "";
  }
  function sessionsForProvider(sessions, providerID) {
    return (sessions || []).filter(item => item.provider === providerID);
  }
  function rememberedSession(sessions, providerID, lastSessions) {
    return sessionsForProvider(sessions, providerID)
      .find(item => item.sessionId === lastSessions?.[providerID]) || null;
  }
  const api = { load, save, availableProvider, sessionsForProvider, rememberedSession };
  if (typeof module !== "undefined" && module.exports) module.exports = api;
  else root.CodeAfarProviderWorkspace = api;
})(typeof globalThis === "undefined" ? this : globalThis);
```

Load this script before `chat.js` in `index.html`, and add it to the embedded asset contract.

- [ ] **Step 4: Verify GREEN**

Run:

```bash
node --test web/chat/provider-workspace.test.js
node --check web/chat/provider-workspace.js
go test ./web
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add web/chat/provider-workspace.js web/chat/provider-workspace.test.js web/chat/index.html web/assets_test.go
git commit -m "test: define provider workspace state"
```

---

### Task 3: Integrate the Claude/Codex Workspace Switcher in Web/Mac/Android

**Files:**
- Modify: `web/design_regression_test.go`
- Modify: `web/chat/index.html`
- Modify: `web/chat/chat.js`
- Modify: `web/chat/desktop.css`
- Modify: `web/chat/mobile.css`

**Interfaces:**
- Consumes: `CodeAfarProviderWorkspace` from Task 2 and existing `provider_list`/`session_list` messages.
- Produces: `state.activeProvider`, `state.lastSessions`, `switchProvider(providerID)`, `visibleSessions()`, two synchronized switcher containers, and provider-specific new-session requests.

- [ ] **Step 1: Write failing layout and behavior contracts**

Add a Go regression test that requires:

```go
func TestProviderSwitcherOwnsNewSessionAndHistory(t *testing.T) {
    htmlBytes, _ := fs.ReadFile(Assets, "chat/index.html")
    jsBytes, _ := fs.ReadFile(Assets, "chat/chat.js")
    cssBytes, _ := fs.ReadFile(Assets, "chat/desktop.css")
    html, js, css := string(htmlBytes), string(jsBytes), string(cssBytes)
    for _, marker := range []string{
        `class="provider-toolbar"`, `id="provider-switcher"`, `id="provider-switcher-mobile"`,
        `aria-label="在当前引擎中新建会话"`, `function switchProvider(providerID)`,
        `sessionsForProvider(state.sessions, state.activeProvider)`,
        `provider: state.activeProvider`, `lastSessions`,
    } {
        if !strings.Contains(html+js+css, marker) { t.Errorf("provider workspace missing %q", marker) }
    }
    for _, forbidden := range []string{`id="draft-provider"`, `id="provider-label"`} {
        if strings.Contains(html, forbidden) { t.Errorf("duplicate composer provider control remains %q", forbidden) }
    }
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `go test ./web -run TestProviderSwitcherOwnsNewSessionAndHistory -count=1`

Expected: FAIL on the missing toolbar, active-provider state, and filtered history.

- [ ] **Step 3: Add the shared toolbar markup and styles**

Use this sidebar structure:

```html
<div class="provider-toolbar">
  <div id="provider-switcher" class="provider-switcher" role="group" aria-label="编码引擎"></div>
  <button id="new-session" class="new-session-button" aria-label="在当前引擎中新建会话" title="新建会话">＋</button>
</div>
```

Add `provider-switcher-mobile` next to `new-session-mobile` in the mobile header. Style `.provider-toolbar` as a single flex row, `.provider-switcher` as a two-segment control, `.provider-option.active` with the accent color, and the plus button as a square peer control. Remove `draft-provider` and `provider-label` from the composer and their responsive rules.

- [ ] **Step 4: Integrate provider state and filtered navigation**

Initialize and persist workspace state:

```javascript
  const providerWorkspace = globalThis.CodeAfarProviderWorkspace;
  const savedWorkspace = providerWorkspace.load(globalThis.localStorage);
  const state = {
    // existing fields...
    activeProvider: savedWorkspace.activeProvider || "claude",
    lastSessions: savedWorkspace.lastSessions,
  };
  function saveWorkspace() {
    providerWorkspace.save(globalThis.localStorage, {
      activeProvider: state.activeProvider, lastSessions: state.lastSessions,
    });
  }
  function visibleSessions() {
    return providerWorkspace.sessionsForProvider(state.sessions, state.activeProvider);
  }
```

Render both switchers from `state.providers`; buttons call `switchProvider`, are disabled when unavailable, set `title` to the unavailable reason, and use `aria-pressed` for the selected provider. Implement switching with this transition:

```javascript
  async function switchProvider(providerID) {
    const descriptor = state.providers.find(item => item.id === providerID);
    if (!descriptor?.available || providerID === state.activeProvider) return;
    if (isDraft() && prompt.value.trim() && !await requestConfirmation("当前新会话草稿尚未发送，确认切换引擎？")) return;
    if (state.selectedSession) state.lastSessions[state.selectedSession.provider] = state.selectedSession.sessionId;
    state.activeProvider = providerID;
    saveWorkspace();
    renderProviderSwitchers();
    renderSessions();
    const remembered = providerWorkspace.rememberedSession(state.sessions, providerID, state.lastSessions);
    if (remembered) await selectSession(remembered.sessionId, remembered.name, false);
    else await beginDraft(false);
  }
```

Change `beginDraft(confirmDiscard = true)` and `selectSession(sessionId, name, confirmDiscard = true)` so switch transitions avoid duplicate confirmation. `selectSession` must set `activeProvider` from the selected session and remember its ID. `renderSessions` and the mobile `<select>` must iterate `visibleSessions()`. `currentProvider()` and `sendCreateRequest()` must use `state.activeProvider`. On `provider_list`, normalize through `availableProvider`; on `session_created`, use `msg.provider`, remember the new ID, and persist. On session stop or refreshed session lists, clear stale remembered IDs and remain within the active provider.

- [ ] **Step 5: Verify the Web integration**

Run:

```bash
go test ./web
node --check web/chat/chat.js
node --test web/chat/provider-workspace.test.js
git diff --check
```

Expected: all pass; the composer contains project and permission controls but no provider selector.

- [ ] **Step 6: Commit**

```bash
git add web
git commit -m "feat: add provider workspaces to chat"
```

---

### Task 4: Mirror Provider Workspaces in the Native iOS Client

**Files:**
- Modify: `ios/Shared/ProtocolModels.swift`
- Modify: `ios/ClaudePhone/Stores/SessionStore.swift`
- Modify: `ios/ClaudePhone/Views/SessionListView.swift`
- Modify: `ios/ClaudePhone/Views/NewSessionView.swift`
- Modify: `ios/ClaudePhoneTests/ProtocolModelsTests.swift`
- Create: `ios/ClaudePhoneTests/SessionStoreTests.swift`

**Interfaces:**
- Produces: `ProviderInfo`, `ProviderPermission`, provider-bearing `SessionInfo`, `ServerMessage.providerList`, `SessionStore.activeProvider`, `visibleSessions`, `switchProvider(_:)`, and provider-specific permission choices.
- Consumes: existing server fields `provider`, `permissionMode`, `providers`, and `permissions`.

- [ ] **Step 1: Write failing protocol and store tests**

Add a protocol decode assertion:

```swift
func testDecodesProviderAndProviderBearingSession() throws {
    let provider = try JSONDecoder().decode(ServerMessage.self, from: Data(#"{"type":"provider_list","providers":[{"id":"codex","name":"Codex","available":true,"permissions":[]}]}"#.utf8))
    XCTAssertEqual(provider, .providerList([ProviderInfo(id: "codex", name: "Codex", available: true, unavailableReason: nil, permissions: [])]))
}
```

Create `SessionStoreTests.swift` with a unique `UserDefaults` suite and verify:

```swift
@MainActor final class SessionStoreTests: XCTestCase {
    func testProviderSwitchFiltersAndRestoresSessions() {
        let defaults = UserDefaults(suiteName: UUID().uuidString)!
        let store = SessionStore(socket: WebSocketClient(), defaults: defaults)
        store.handle(.providerList([
            ProviderInfo(id: "claude", name: "Claude", available: true, unavailableReason: nil, permissions: []),
            ProviderInfo(id: "codex", name: "Codex", available: true, unavailableReason: nil, permissions: []),
        ]))
        store.handle(.sessionList([
            SessionInfo(sessionId: "c1", name: "Claude task", status: "active", owner: "Mac", subscribers: [], createdAt: 1, cwd: "/c", provider: "claude", model: nil, permissionMode: "default"),
            SessionInfo(sessionId: "x1", name: "Codex task", status: "active", owner: "Mac", subscribers: [], createdAt: 2, cwd: "/x", provider: "codex", model: nil, permissionMode: "workspaceWrite"),
        ]))
        store.select(store.sessions[0])
        store.switchProvider("codex")
        XCTAssertEqual(store.visibleSessions.map(\.sessionId), ["x1"])
        store.select(store.sessions[1])
        store.switchProvider("claude")
        XCTAssertEqual(store.selectedSessionID, "c1")
    }
}
```

- [ ] **Step 2: Run XCTest and verify RED**

Run:

```bash
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild test -quiet -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=macOS,arch=arm64,variant=Designed for [iPad,iPhone]' CODE_SIGNING_ALLOWED=NO -only-testing:ClaudePhoneTests/ProtocolModelsTests -only-testing:ClaudePhoneTests/SessionStoreTests
```

Expected: compile/test failure because provider models and store APIs do not exist.

- [ ] **Step 3: Extend protocol models without changing the wire format**

Add Codable/Hashable `ProviderPermission` and `ProviderInfo`, add `cwd`, `provider`, `model`, and `permissionMode` to `SessionInfo`, and decode `provider_list`. Expand `sessionCreated` to carry `provider`, `model`, and `permissionMode`, matching `pkg/protocol/messages.go` exactly. Update all pattern matches and test initializers to the new associated values.

- [ ] **Step 4: Implement iOS workspace state**

Inject `UserDefaults` into `SessionStore`, expose `providers`, `activeProvider`, and:

```swift
var visibleSessions: [SessionInfo] { sessions.filter { $0.provider == activeProvider } }
var activeProviderInfo: ProviderInfo? { providers.first { $0.id == activeProvider } }

func switchProvider(_ id: String) {
    guard providers.first(where: { $0.id == id })?.available == true else { return }
    if let selected = sessions.first(where: { $0.sessionId == selectedSessionID }) {
        lastSessions[selected.provider] = selected.sessionId
    }
    activeProvider = id
    selectedSessionID = sessions.first { $0.provider == id && $0.sessionId == lastSessions[id] }?.sessionId
    persistWorkspace()
    if let selectedSessionID, let selected = sessions.first(where: { $0.sessionId == selectedSessionID }) { select(selected) }
}
```

Request `list_providers` on hello. Make `create` include `"provider": activeProvider`; build the optimistic session from all `session_created` fields. Normalize an unavailable saved provider to the first available provider after `provider_list` arrives. Persist only active provider and last session IDs.

- [ ] **Step 5: Build the native provider row and provider-specific draft**

In `SessionListView`, replace the bare list with a `VStack` containing an `HStack`: a segmented-looking row of provider buttons and the adjacent plus button. Each provider button displays its server name, is disabled when unavailable, and calls `switchProvider`. Iterate `visibleSessions`, and change the empty-state description to refer to the selected provider.

In `NewSessionView`, remove hard-coded Claude permissions and iterate:

```swift
ForEach(store.activeProviderInfo?.permissions ?? [], id: \.id) { option in
    Text(option.dangerous ? "\(option.label) ⚠" : option.label).tag(option.id)
}
```

Reset the selected permission to the active provider's first/default option when the sheet appears or active provider changes.

- [ ] **Step 6: Verify iOS behavior**

Run:

```bash
DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer xcodebuild test -quiet -project ios/ClaudePhone.xcodeproj -scheme ClaudePhone -destination 'platform=macOS,arch=arm64,variant=Designed for [iPad,iPhone]' CODE_SIGNING_ALLOWED=NO
./scripts/validate-ios-project.sh
```

Expected: all XCTest cases pass and project validation ends with `iOS project structure OK`.

- [ ] **Step 7: Commit**

```bash
git add ios
git commit -m "feat: add provider workspaces to iOS"
```

---

### Task 5: Full Verification, Mac Installation, and Delivery

**Files:**
- Modify only if a verification failure exposes a requirement regression; add its failing test before the correction.
- Verify: `/Applications/CodeAfar.app`

**Interfaces:**
- Consumes: completed Web and iOS changes from Tasks 1–4.
- Produces: tested, installed, committed, and pushed CodeAfar build.

- [ ] **Step 1: Run the complete automated suite**

Run:

```bash
go test ./...
go test -race ./pkg/engine
node --check web/chat/chat.js
node --check web/chat/provider-workspace.js
node --check web/admin/admin.js
node --test web/chat/provider-workspace.test.js
./scripts/validate-ios-project.sh
git diff --check
```

Expected: every command exits 0 without warnings attributable to the change.

- [ ] **Step 2: Build, verify, and install the Mac application**

Run:

```bash
make install-mac-app
./scripts/test-mac-reopen.sh
```

Expected: `build/CodeAfar.app` passes bundle verification, `/Applications/CodeAfar.app` launches, the local status endpoint reports ready, and reopen testing passes.

- [ ] **Step 3: Exercise the acceptance matrix against the installed app**

Verify all of the following on the installed bundle:

```text
1. Claude | Codex and + share the sidebar's first row.
2. Switching providers changes the history list and never leaves an opposite-provider chat open.
3. Each provider restores its own last session after switching and after relaunch.
4. + creates a draft for the selected provider without a second provider picker.
5. A Claude Read/Bash task and a Codex command task complete without showing tool names or raw commands.
6. Old history containing tool_use events also hides those events.
7. Return sends; Shift+Return inserts a newline; message selection and copy still work.
8. Strict/review/full-access permission choices match the selected provider and still take effect.
```

Expected: all eight checks pass. Capture the exact failure and add a regression test before changing code if any check fails.

- [ ] **Step 4: Review the final diff and commit any acceptance-only fixes**

Run:

```bash
git status --short
git diff --stat HEAD~4..HEAD
git diff --check
```

If Task 5 produced tested changes, commit them with:

```bash
git add -A
git commit -m "fix: complete provider workspace acceptance"
```

Expected: working tree clean.

- [ ] **Step 5: Push delivery**

Run:

```bash
git push origin master
```

Expected: remote `master` advances to the final verified commit.
