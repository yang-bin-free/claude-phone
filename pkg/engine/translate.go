package engine

import (
	"encoding/json"
	"strings"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func translateClaudeOutput(payload []byte) [][]byte {
	var raw struct {
		Type    string   `json:"type"`
		Subtype string   `json:"subtype"`
		IsError bool     `json:"is_error"`
		Result  string   `json:"result"`
		Errors  []string `json:"errors"`
		Event   struct {
			Type  string                      `json:"type"`
			Delta struct{ Type, Text string } `json:"delta"`
		} `json:"event"`
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
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
		}
	case "assistant":
		messages := make([][]byte, 0, len(raw.Message.Content))
		for _, block := range raw.Message.Content {
			if block.Type != "tool_use" || block.Name == "" {
				continue
			}
			input := strings.TrimSpace(string(block.Input))
			if input == "" || input == "null" {
				continue
			}
			messages = append(messages, marshalTranslated(protocol.ToolUseMsg{
				Type: protocol.TypeToolUse, Tool: block.Name, Input: input,
			})...)
		}
		return messages
	case "result":
		if raw.IsError {
			message := strings.TrimSpace(raw.Result)
			if message == "" {
				parts := make([]string, 0, len(raw.Errors))
				for _, item := range raw.Errors {
					if item = strings.TrimSpace(item); item != "" {
						parts = append(parts, item)
					}
				}
				message = strings.Join(parts, "; ")
			}
			if message == "" {
				message = "Claude exited with an unspecified error"
			}
			return marshalTranslated(protocol.NewError("CLAUDE_ERROR", message))
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
