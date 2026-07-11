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
	for _, marker := range []string{"params.get(\"ws\")", "params.get(\"deviceToken\")", "params.get(\"platform\")"} {
		if !strings.Contains(string(js), marker) {
			t.Fatalf("chat.js missing %q", marker)
		}
	}
}
