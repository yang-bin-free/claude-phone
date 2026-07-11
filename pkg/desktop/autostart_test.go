package desktop

import (
	"strings"
	"testing"
)

func TestLaunchAgentXMLEscapesExecutableAndArguments(t *testing.T) {
	b, err := launchAgentXML("/Applications/Claude & Phone.app/run", []string{"--data-dir", "/tmp/a<b"})
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{LaunchAgentLabel, "/Applications/Claude &amp; Phone.app/run", "/tmp/a&lt;b", "<key>RunAtLoad</key><true></true>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plist missing %q: %s", want, text)
		}
	}
}
