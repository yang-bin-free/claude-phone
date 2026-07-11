package main

import (
	"strings"
	"testing"
)

func TestGeneratePairingKey(t *testing.T) {
	key, err := generatePairingKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	if !strings.HasPrefix(key, "pk_") {
		t.Fatalf("key prefix mismatch: %q", key)
	}
	if len(key) <= len("pk_") {
		t.Fatalf("key too short: %q", key)
	}
}
