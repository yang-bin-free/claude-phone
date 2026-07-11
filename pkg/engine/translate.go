package engine

import (
	"encoding/json"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func translateClaudeOutput(payload []byte) [][]byte {
	var raw struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype"`
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
		Event   struct {
			Type         string                      `json:"type"`
			Delta        struct{ Type, Text string } `json:"delta"`
			ContentBlock struct {
				Type, Name string
				Input      json.RawMessage
			} `json:"content_block"`
		} `json:"event"`
	}
	if json.Unmarshal(payload, &raw) != nil {
		return nil
	}
	switch raw.Type {
	case protocol.TypeThinking, protocol.TypeToken, protocol.TypeToolUse, protocol.TypeDone, protocol.TypeError:
		return [][]byte{payload}
	case "system":
		if raw.Subtype == "init" {
			return marshalTranslated(protocol.ThinkingMsg{Type: protocol.TypeThinking})
		}
	case "stream_event":
		switch raw.Event.Type {
		case "content_block_delta":
			if raw.Event.Delta.Type == "text_delta" && raw.Event.Delta.Text != "" {
				return marshalTranslated(protocol.TokenMsg{Type: protocol.TypeToken, Content: raw.Event.Delta.Text})
			}
		case "content_block_start":
			if raw.Event.ContentBlock.Type == "tool_use" {
				return marshalTranslated(protocol.ToolUseMsg{Type: protocol.TypeToolUse, Tool: raw.Event.ContentBlock.Name, Input: string(raw.Event.ContentBlock.Input)})
			}
		}
	case "result":
		if raw.IsError {
			return marshalTranslated(protocol.NewError("CLAUDE_ERROR", raw.Result))
		}
		return marshalTranslated(protocol.DoneMsg{Type: protocol.TypeDone})
	}
	return nil
}

func marshalTranslated(value any) [][]byte {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return [][]byte{b}
}
