package web

import (
	"io/fs"
	"strings"
	"testing"
)

func TestSharedChatAssetsSupportMobileRemoteConnection(t *testing.T) {
	for _, name := range []string{
		"chat/index.html", "chat/chat.js", "chat/core.css", "chat/desktop.css", "chat/mobile.css",
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
