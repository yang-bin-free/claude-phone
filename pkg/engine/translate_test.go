package engine

import (
	"encoding/json"
	"testing"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
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

func TestTranslateClaudeToolUseWaitsForCompleteAssistantEvent(t *testing.T) {
	start := []byte(`{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tool-1","name":"Read","input":{}}}}`)
	if got := translateClaudeOutput(start); len(got) != 0 {
		t.Fatalf("incomplete tool start translated as %q", got)
	}
	partial := []byte(`{"type":"stream_event","event":{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"file_path\":\"/tmp/README.md\"}"}}}`)
	if got := translateClaudeOutput(partial); len(got) != 0 {
		t.Fatalf("partial tool input translated as %q", got)
	}

	complete := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool-1","name":"Read","input":{"file_path":"/tmp/README.md","limit":1}}]}}`)
	got := translateClaudeOutput(complete)
	if len(got) != 1 {
		t.Fatalf("complete tool use translated as %q", got)
	}
	var message protocol.ToolUseMsg
	if err := json.Unmarshal(got[0], &message); err != nil {
		t.Fatal(err)
	}
	if message.Tool != "Read" || message.Input != `{"file_path":"/tmp/README.md","limit":1}` {
		t.Fatalf("translated tool use = %+v", message)
	}
}

func TestTranslateClaudeToolUseRejectsMissingInput(t *testing.T) {
	for _, input := range []string{
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":null}]}}`,
	} {
		if got := translateClaudeOutput([]byte(input)); len(got) != 0 {
			t.Fatalf("tool use without complete input translated as %q", got)
		}
	}
}

func TestTranslateClaudeErrorFallsBackToErrorsArray(t *testing.T) {
	got := translateClaudeOutput([]byte(`{"type":"result","is_error":true,"result":"","errors":["No conversation found with session ID: abc"]}`))
	if len(got) != 1 {
		t.Fatalf("translated error = %q", got)
	}
	var message protocol.ErrorMsg
	if err := json.Unmarshal(got[0], &message); err != nil {
		t.Fatal(err)
	}
	if message.Code != "CLAUDE_ERROR" || message.Message != "No conversation found with session ID: abc" {
		t.Fatalf("translated error = %+v", message)
	}
}

func TestTranslateClaudeErrorNeverProducesEmptyMessage(t *testing.T) {
	got := translateClaudeOutput([]byte(`{"type":"result","is_error":true}`))
	if len(got) != 1 {
		t.Fatalf("translated error = %q", got)
	}
	var message protocol.ErrorMsg
	if err := json.Unmarshal(got[0], &message); err != nil {
		t.Fatal(err)
	}
	if message.Message == "" {
		t.Fatal("translated Claude error message is empty")
	}
}

func TestTranslatePreservesFakeProtocolMessages(t *testing.T) {
	input := []byte(`{"type":"token","content":"chunk"}`)
	got := translateClaudeOutput(input)
	if len(got) != 1 || string(got[0]) != string(input) {
		t.Fatalf("translated = %q", got)
	}
}
