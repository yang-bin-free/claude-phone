package product

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserVisibleProductFilesUseCodeAfarBrand(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	paths := []string{
		"web/chat/index.html",
		"scripts/Info.plist",
		"android/app/src/main/AndroidManifest.xml",
		"ios/ClaudePhone/Info.plist",
		"ios/ClaudePhoneTunnel/Info.plist",
	}
	for _, name := range paths {
		b, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if strings.Contains(text, "Claude Phone") || !strings.Contains(text, Name) {
			t.Errorf("%s does not use the CodeAfar brand", name)
		}
	}
}

func TestMacBuildFilesProduceCodeAfarArtifacts(t *testing.T) {
	repo := filepath.Clean(filepath.Join("..", ".."))
	for _, name := range []string{"scripts/build-mac-app.sh", "scripts/package-release.sh", "Makefile"} {
		b, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !strings.Contains(text, "CodeAfar.app") || strings.Contains(text, "build/Claude Phone.app") {
			t.Errorf("%s still builds the legacy app", name)
		}
	}
}
