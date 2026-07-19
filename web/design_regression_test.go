package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestDesktopAdminFormsHavePersistentLabelsAndMode(t *testing.T) {
	htmlBytes, err := fs.ReadFile(Assets, "chat/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, id := range []string{
		"device-name", "project-name", "project-path", "template-label",
		"template-prompt", "permission-tool", "permission-pattern",
	} {
		if !strings.Contains(html, `for="`+id+`"`) {
			t.Errorf("admin control %s has no persistent label", id)
		}
	}

	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	if !strings.Contains(js, `classList.toggle("admin-mode"`) {
		t.Error("chat shell does not expose admin mode to presentation")
	}
}

func TestDesktopDiagnosticsUseCompactSummary(t *testing.T) {
	htmlBytes, err := fs.ReadFile(Assets, "chat/index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlBytes), `class="metric-summary"`) {
		t.Error("diagnostics should use one compact summary region")
	}

	jsBytes, err := fs.ReadFile(Assets, "admin/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(jsBytes), `class="metric-item"`) {
		t.Error("diagnostics items should use the flat summary presentation")
	}
}

func TestDesktopThemeUsesWarmAccentAndMacSizedControls(t *testing.T) {
	cssBytes, err := fs.ReadFile(Assets, "chat/core.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssBytes)
	for _, marker := range []string{"--accent: #d97757", "min-height: 44px", "var(--accent)"} {
		if !strings.Contains(css, marker) {
			t.Errorf("desktop theme missing %q", marker)
		}
	}
	if strings.Contains(css, "#7c5cff") {
		t.Error("legacy generic-purple accent is still active")
	}
}

func TestDesktopDestructiveActionsRequireConfirmation(t *testing.T) {
	adminBytes, err := fs.ReadFile(Assets, "admin/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	admin := string(adminBytes)
	for _, marker := range []string{
		"confirmDangerousAction", "确认吊销这个设备", "确认删除这个工作目录",
		"确认删除这个提示词模板", "确认删除这条权限规则", "确认停止这个会话",
	} {
		if !strings.Contains(admin, marker) {
			t.Errorf("admin destructive flow missing %q", marker)
		}
	}

	chatBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(chatBytes), "确认停止当前会话") {
		t.Error("chat session stop does not ask for confirmation")
	}
}

func TestDesktopEmptyStateExplainsTheNextAction(t *testing.T) {
	htmlBytes, err := fs.ReadFile(Assets, "chat/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, marker := range []string{"选择工作目录", "⌘N", `class="empty-title"`} {
		if !strings.Contains(html, marker) {
			t.Errorf("desktop empty state missing %q", marker)
		}
	}
}

func TestDesktopComposerWaitsForSessionSelectionAfterReconnect(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	for _, marker := range []string{
		"sessionReady: false", "function updateControls()", "state.sessionReady = false",
		"state.sessionReady = true", `action: "select_session", sessionId: state.sessionId`,
	} {
		if !strings.Contains(js, marker) {
			t.Errorf("chat reconnect guard missing %q", marker)
		}
	}
}

func TestDesktopMessagesAreSelectable(t *testing.T) {
	cssBytes, err := fs.ReadFile(Assets, "chat/desktop.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssBytes)
	marker := ".message { position: relative; max-width: 78%; margin: 0 0 16px; padding: 12px 64px 12px 15px; border-radius: 14px; white-space: pre-wrap; user-select: text; cursor: text; }"
	if !strings.Contains(css, marker) {
		t.Fatalf("message selection rule missing %q", marker)
	}
}

func TestDesktopMessagesExposeCopyAction(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	for _, marker := range []string{
		`button.className = "message-copy"`,
		`button.setAttribute("aria-label", "复制消息")`,
		`await writeClipboard(content.textContent)`,
		`document.execCommand("copy")`,
	} {
		if !strings.Contains(js, marker) {
			t.Errorf("message copy action missing %q", marker)
		}
	}
}

func TestDesktopReturnSendsWithoutBreakingIME(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	marker := `prompt.addEventListener("keydown", event => {
    if (event.isComposing || event.keyCode === 229) return;
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      composer.requestSubmit();
    }
  });`
	if !strings.Contains(js, marker) {
		t.Fatal("composition-safe Return-to-send handler missing")
	}
}
