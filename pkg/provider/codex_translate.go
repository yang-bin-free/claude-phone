package provider

import (
	"encoding/json"
	"strings"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
)

func (a *CodexAdapter) TranslateOutput(payload []byte) [][]byte {
	var event struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
		Item struct {
			Type      string          `json:"type"`
			Text      string          `json:"text"`
			Command   string          `json:"command"`
			Server    string          `json:"server"`
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
			Query     string          `json:"query"`
			Changes   []struct {
				Path string `json:"path"`
				Kind string `json:"kind"`
			} `json:"changes"`
		} `json:"item"`
	}
	if json.Unmarshal(payload, &event) != nil {
		return nil
	}
	switch event.Type {
	case protocol.TypeThinking, protocol.TypeToken, protocol.TypeToolUse, protocol.TypeDone:
		return [][]byte{payload}
	case protocol.TypeError:
		if event.Code != "" {
			return [][]byte{payload}
		}
		return marshalCodex(protocol.NewError("CODEX_ERROR", nonEmptyCodexError(event.Message)))
	case "item.started":
		switch event.Item.Type {
		case "command_execution":
			if event.Item.Command == "" {
				return nil
			}
			return marshalCodexTool("Bash", map[string]any{"command": event.Item.Command})
		case "mcp_tool_call":
			name := strings.Trim(strings.Join([]string{event.Item.Server, event.Item.Tool}, "/"), "/")
			if name == "" {
				name = "工具"
			}
			input := strings.TrimSpace(string(event.Item.Arguments))
			if input == "" || input == "null" {
				input = "{}"
			}
			return marshalCodex(protocol.ToolUseMsg{Type: protocol.TypeToolUse, Tool: "MCP · " + name, Input: input})
		case "web_search":
			if event.Item.Query == "" {
				return nil
			}
			return marshalCodexTool("WebSearch", map[string]any{"query": event.Item.Query})
		}
	case "item.completed":
		switch event.Item.Type {
		case "agent_message":
			if event.Item.Text != "" {
				return marshalCodex(protocol.TokenMsg{Type: protocol.TypeToken, Content: event.Item.Text})
			}
		case "file_change":
			messages := make([][]byte, 0, len(event.Item.Changes))
			for _, change := range event.Item.Changes {
				if change.Path == "" {
					continue
				}
				tool := "Edit"
				switch strings.ToLower(change.Kind) {
				case "add", "create", "write":
					tool = "Write"
				case "delete", "remove":
					tool = "Delete"
				}
				messages = append(messages, marshalCodexTool(tool, map[string]any{"file_path": change.Path})...)
			}
			return messages
		}
	case "turn.completed":
		return marshalCodex(protocol.DoneMsg{Type: protocol.TypeDone})
	case "turn.failed":
		message := event.Error.Message
		if message == "" {
			message = event.Message
		}
		return marshalCodex(protocol.NewError("CODEX_ERROR", nonEmptyCodexError(message)))
	}
	return nil
}

func marshalCodexTool(tool string, input any) [][]byte {
	b, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	return marshalCodex(protocol.ToolUseMsg{Type: protocol.TypeToolUse, Tool: tool, Input: string(b)})
}

func marshalCodex(value any) [][]byte {
	b, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return [][]byte{b}
}

func nonEmptyCodexError(message string) string {
	if message = strings.TrimSpace(message); message != "" {
		return message
	}
	return "Codex exited with an unspecified error"
}
