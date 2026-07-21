package product

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIOSProviderWorkspaceMatchesSharedChatBehavior(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	models := readContractFile(t, repo, "ios/Shared/ProtocolModels.swift")
	store := readContractFile(t, repo, "ios/ClaudePhone/Stores/SessionStore.swift")
	view := readContractFile(t, repo, "ios/ClaudePhone/Views/SessionListView.swift")
	newSession := readContractFile(t, repo, "ios/ClaudePhone/Views/NewSessionView.swift")
	for _, marker := range []string{
		`struct ProviderInfo`, `let provider: String`, `case providerList`,
		`var activeProvider`, `var visibleSessions`, `func switchProvider(_ id: String)`,
		`"provider": activeProvider`, `List(app.sessions.visibleSessions)`,
		`ForEach(store.activeProviderInfo?.permissions ?? []`,
	} {
		if !strings.Contains(models+store+view+newSession, marker) {
			t.Errorf("iOS provider workspace missing %q", marker)
		}
	}
}
