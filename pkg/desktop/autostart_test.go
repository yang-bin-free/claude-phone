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
	for _, want := range []string{LaunchAgentLabel, "/Applications/Claude &amp; Phone.app/run", "/tmp/a&lt;b", "<key>RunAtLoad</key><true/>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plist missing %q: %s", want, text)
		}
	}
}

func TestLaunchAgentXMLUsesLaunchdCompatibleBooleanElements(t *testing.T) {
	b, err := launchAgentXML("/Applications/Claude Phone.app/run", nil)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"<key>RunAtLoad</key><true/>", "<key>KeepAlive</key><false/>"} {
		if !strings.Contains(text, want) {
			t.Fatalf("plist missing launchd-compatible boolean %q: %s", want, text)
		}
	}
	for _, invalid := range []string{"<true></true>", "<false></false>"} {
		if strings.Contains(text, invalid) {
			t.Fatalf("plist contains launchd-incompatible boolean %q: %s", invalid, text)
		}
	}
}
