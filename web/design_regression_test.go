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
