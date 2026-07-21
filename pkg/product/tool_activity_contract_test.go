package product

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIOSChatDoesNotRenderInternalToolActivity(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	chatStore := readContractFile(t, repo, "ios/ClaudePhone/Stores/ChatStore.swift")
	for _, forbidden := range []string{`append(.tool`, `"🔧 \(tool)`, `case "tool_use": append(.tool`} {
		if strings.Contains(chatStore, forbidden) {
			t.Errorf("iOS chat still renders internal tool activity %q", forbidden)
		}
	}
	for _, marker := range []string{`case .toolUse: break`, `case "tool_use": break`} {
		if !strings.Contains(chatStore, marker) {
			t.Errorf("iOS chat does not explicitly ignore tool activity %q", marker)
		}
	}
}
