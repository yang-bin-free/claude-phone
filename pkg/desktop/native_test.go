package desktop

import (
	"net/url"
	"testing"
)

func TestURLWithAdminTokenUsesFragment(t *testing.T) {
	got, err := URLWithAdminToken("http://127.0.0.1:9877/", "secret value")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "token=secret+value" {
		t.Fatalf("url=%q", got)
	}
}
