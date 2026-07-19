# Mac Chat Usability and Session Recovery Design

## Scope

Fix three Mac V1 failures reported from the installed application:

1. Every rendered chat message must support native text selection and a reliable per-message copy action.
2. Return sends, Shift+Return inserts a newline, and IME composition Return never sends.
3. Selecting a persisted Claude Phone session whose Claude CLI transcript no longer exists must start a usable replacement process instead of invoking a guaranteed-to-fail `--resume`.

## Root Cause

The installed app was launched by macOS with `/` as its process working directory. The selected persisted Claude Phone session had metadata and UI history but no matching Claude CLI transcript. `resumeSession` unconditionally set `Resume: true`, so Claude returned `No conversation found with session ID`. The output translator only read `result`, while Claude 2.1.212 placed the diagnostic in `errors[]`, producing an empty `CLAUDE_ERROR`. After Claude exited, the engine retained a dead process entry.

The chat stylesheet only opted assistant and tool messages into `user-select: text`. The prompt key handler only submitted on Command+Return.

## Chosen Design

### Session recovery

Add a session-package query that searches the active Claude configuration directory for a transcript named with the persisted session UUID. It uses `CLAUDE_CONFIG_DIR` when set and otherwise `~/.claude`. A restored session is launched with `--resume` only when that transcript exists. If it does not exist, it starts with `--session-id` using the existing UUID, preserving Claude Phone history and making the next prompt usable without user cleanup.

The engine owns an injectable `sessionExists` function so resume behavior can be tested without depending on the developer machine. New sessions are unchanged.

The output translator also reads Claude's `errors[]`; when `result` is empty it joins the non-empty error strings. This preserves actionable diagnostics for any future Claude execution failure.

### Copy behavior

Apply `user-select: text` and a text cursor to every `.message` bubble, including user, assistant, tool, queued, and error messages. Each bubble also gets an accessible hover/focus copy button. The button first uses the Clipboard API and falls back to a temporary selected textarea plus `execCommand("copy")`, covering WKWebView environments where the exposed Clipboard API is denied. A short in-place “已复制” state confirms success without changing message text.

### Keyboard behavior

The prompt key handler submits on Return when Shift is not pressed. It calls `preventDefault()` so a newline is not inserted before submission. Shift+Return retains the textarea's native multiline behavior. Events with `isComposing` or legacy key code 229 are ignored to protect Chinese and Japanese IME confirmation.

## Error Handling

- A missing transcript is treated as recoverable and starts a fresh Claude process transparently.
- Other Claude result errors remain visible, now with their actual `errors[]` diagnostic.
- If a transcript existence check itself cannot read the filesystem, it returns false and takes the safer fresh-session path.
- No existing Claude Phone history file is deleted or rewritten.

## Tests

1. Session transcript lookup tests cover default/configured roots, present and missing UUIDs.
2. Engine resume tests capture `ClaudeConfig` and assert `Resume` is false for a missing transcript and true for a present transcript.
3. Translator tests assert `errors[]` becomes a non-empty `CLAUDE_ERROR` message.
4. Web asset regression tests assert selectable message CSS, the copy action/fallback, and Return/Shift+Return/IME guards.
5. Browser E2E verifies clipboard contents, Return submission, Shift+Return newline, and exact single response.
6. Installed-app E2E reuses the currently failing persisted session, sends `你好`, receives a real Claude response, then audits child-process cleanup.

## Non-goals

- Deleting or migrating the user's existing Claude Phone history.
- Reactive process restart and automatic replay for arbitrary mid-turn Claude crashes.
