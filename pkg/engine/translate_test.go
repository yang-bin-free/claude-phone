package engine

import (
	"encoding/json"
	"testing"
)

func TestTranslateClaudeStreamEvents(t *testing.T) {
	tests := []struct{ input, wantType, wantContent string }{
		{`{"type":"system","subtype":"init"}`, "thinking", ""},
		{`{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"你好"}}}`, "token", "你好"},
		{`{"type":"result","subtype":"success","is_error":false,"result":"你好"}`, "done", ""},
		{`{"type":"result","subtype":"error","is_error":true,"result":"failed"}`, "error", ""},
	}
	for _, tt := range tests {
		messages := translateClaudeOutput([]byte(tt.input))
		if len(messages) != 1 {
			t.Fatalf("translate %s = %q", tt.input, messages)
		}
		var got struct{ Type, Content string }
		if err := json.Unmarshal(messages[0], &got); err != nil {
			t.Fatal(err)
		}
		if got.Type != tt.wantType || got.Content != tt.wantContent {
			t.Fatalf("got %+v want type=%s content=%s", got, tt.wantType, tt.wantContent)
		}
	}
}

func TestTranslatePreservesFakeProtocolMessages(t *testing.T) {
	input := []byte(`{"type":"token","content":"chunk"}`)
	got := translateClaudeOutput(input)
	if len(got) != 1 || string(got[0]) != string(input) {
		t.Fatalf("translated = %q", got)
	}
}
