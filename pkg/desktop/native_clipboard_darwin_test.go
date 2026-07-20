//go:build darwin

package desktop

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestNativeClipboardCopiesUnicodeText(t *testing.T) {
	name := fmt.Sprintf("com.codeafar.tests.%d.%d", os.Getpid(), time.Now().UnixNano())
	t.Cleanup(func() { releaseNativePasteboard(name) })
	const want = "CodeAfar 复制测试：你好 👋"
	if !writeNativeClipboardToPasteboard(want, name) {
		t.Fatal("native clipboard rejected text")
	}
	got, ok := readNativeClipboardFromPasteboard(name)
	if !ok || got != want {
		t.Fatalf("clipboard=%q want %q", got, want)
	}
}
