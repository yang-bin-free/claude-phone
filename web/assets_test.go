package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSharedChatAssetsSupportMobileRemoteConnection(t *testing.T) {
	for _, name := range []string{
		"chat/index.html", "chat/chat.js", "chat/tool-format.js", "chat/core.css", "chat/desktop.css", "chat/mobile.css",
	} {
		if _, err := fs.ReadFile(Assets, name); err != nil {
			t.Fatalf("missing embedded asset %s: %v", name, err)
		}
	}
	js, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	adminJS, err := fs.ReadFile(Assets, "admin/admin.js")
	if err != nil {
		t.Fatal(err)
	}
	scripts := string(js) + string(adminJS)
	for _, marker := range []string{
		"params.get(\"ws\")", "params.get(\"deviceToken\")", "params.get(\"platform\")",
		"case \"session_list\"", "assistantChunk", "action: \"select_session\"",
		"action: \"stop_session\"", "AndroidBridge.openSettings",
		"action: \"load_history\"", "case \"history\"",
		"case \"health\"", "会话可能卡住",
		"action: \"list_templates\"", "case \"template_list\"",
		"requestAnimationFrame(flushTokens)", "messages.children.length > 500",
		"case \"queued\"", "已排队",
		"case \"tool_use\"", "msg.tool",
		"/admin/devices/", "revokeDevice",
		"/admin/projects", "deleteProject",
		"/admin/permission-rules", "deletePermissionRule",
		"AndroidBridge.startVoice", "setPrompt",
	} {
		if !strings.Contains(scripts, marker) {
			t.Fatalf("shared scripts missing %q", marker)
		}
	}
}

func TestDesktopShellHasStableStateAndNavigationHooks(t *testing.T) {
	htmlBytes, err := fs.ReadFile(Assets, "chat/index.html")
	if err != nil {
		t.Fatal(err)
	}
	html := string(htmlBytes)
	for _, id := range []string{"startup-banner", "chat-view", "admin-view", "session-list", "composer", "admin-sessions"} {
		marker := `id="` + id + `"`
		if strings.Count(html, marker) != 1 {
			t.Fatalf("%s count=%d", marker, strings.Count(html, marker))
		}
	}

	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	for _, marker := range []string{"/desktop/status", "showChat", "showAdmin", "setComposerEnabled", "showBanner", "retryTimer", "JSON.parse"} {
		if !strings.Contains(js, marker) {
			t.Fatalf("chat.js missing %q", marker)
		}
	}
}

func TestAuthorizationFailuresUseSingletonBannerInsteadOfChatHistory(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	for _, marker := range []string{
		`function showProtocolError(msg)`,
		`msg.code === "DEVICE_NOT_AUTHORIZED"`,
		"showBanner(`${msg.code}: ${msg.message}`)",
		`showProtocolError(msg)`,
	} {
		if !strings.Contains(js, marker) {
			t.Fatalf("authorization error deduplication missing %q", marker)
		}
	}
}

func TestStoppedSessionRestoresEngineConnectionLabel(t *testing.T) {
	jsBytes, err := fs.ReadFile(Assets, "chat/chat.js")
	if err != nil {
		t.Fatal(err)
	}
	js := string(jsBytes)
	marker := `connection.textContent = state.connected ? "已连接" : "重新连接中"`
	if !strings.Contains(js, marker) {
		t.Fatalf("stopped-session connection reset missing %q", marker)
	}
}
