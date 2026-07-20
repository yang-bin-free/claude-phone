// Package protocol 定义三端共享的 WebSocket JSON 消息与错误码。
// 对应 README §5.1(手机→Mac) / §5.2(Mac→手机) / §5.3(错误码)。
package protocol

import (
	"bytes"
	"encoding/json"
)

const ProtocolVersion = "2"

// 消息 type 常量（手机→Mac 与 Mac→手机 合并列出）。
const (
	TypeAuth               = "auth"
	TypeControl            = "control"
	TypeText               = "text"
	TypeVoice              = "voice"
	TypePermissionResponse = "permission_response"
	TypePermissionRule     = "permission_rule"

	TypeHello             = "hello"
	TypeProjectList       = "project_list"
	TypeProviderList      = "provider_list"
	TypeTemplateList      = "template_list"
	TypeSessionList       = "session_list"
	TypeSessionCreated    = "session_created"
	TypeSessionSelected   = "session_selected"
	TypeSessionStopped    = "session_stopped"
	TypeThinking          = "thinking"
	TypeToken             = "token"
	TypeToolUse           = "tool_use"
	TypeDone              = "done"
	TypeError             = "error"
	TypeQueued            = "queued"
	TypeDequeued          = "dequeued"
	TypePong              = "pong"
	TypeHealth            = "health"
	TypePermissionChanged = "permission_changed"
	TypeTextAccepted      = "text_accepted"
)

// control action 常量。
const (
	ActionCreateSession = "create_session"
	ActionSelectSession = "select_session"
	ActionJoinSession   = "join_session"
	ActionLeaveSession  = "leave_session"
	ActionStopSession   = "stop_session"
	ActionListSessions  = "list_sessions"
	ActionListProjects  = "list_projects"
	ActionListTemplates = "list_templates"
	ActionListProviders = "list_providers"
	ActionSetPermission = "set_permission_mode"
	ActionCancel        = "cancel"
	ActionLoadHistory   = "load_history"
	ActionPing          = "ping"
	ActionForceKill     = "force_kill"
	ActionWaitLonger    = "wait_longer"
)

// 错误码，对应 README §5.3。
const (
	CodeSessionNotFound       = "SESSION_NOT_FOUND"
	CodeSessionNotOwner       = "SESSION_NOT_OWNER"
	CodeSessionLimitReached   = "SESSION_LIMIT_REACHED"
	CodeDeviceNotAuthorized   = "DEVICE_NOT_AUTHORIZED"
	CodeClaudeNotFound        = "CLAUDE_NOT_FOUND"
	CodeClaudeVersionMismatch = "CLAUDE_VERSION_MISMATCH"
	CodeProviderNotAvailable  = "PROVIDER_NOT_AVAILABLE"
	CodeInvalidPermission     = "INVALID_PERMISSION_MODE"
	CodeRequestIDConflict     = "REQUEST_ID_CONFLICT"
)

// Envelope 是所有入站消息的第一层解析结果。
type Envelope struct {
	Type string
	Raw  json.RawMessage
}

// ParseEnvelope 解出消息 type，保留原始 JSON 供二次解析。
func ParseEnvelope(b []byte) (Envelope, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return Envelope{}, err
	}
	// clone b：入参通常是复用的 WS 读缓冲，Raw 需在读循环外存活，不能别名底层数组
	return Envelope{Type: head.Type, Raw: bytes.Clone(b)}, nil
}

// ---- 入站消息 ----

type AuthMsg struct {
	Type        string `json:"type"`
	DeviceToken string `json:"deviceToken"`
	DeviceName  string `json:"deviceName"`
}

type ControlMsg struct {
	Type           string `json:"type"`
	Action         string `json:"action"`
	SessionID      string `json:"sessionId,omitempty"`
	Name           string `json:"name,omitempty"`
	WorkingDir     string `json:"workingDir,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	RequestID      string `json:"requestId,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
	BeforeMsgID    string `json:"beforeMsgId,omitempty"`
}

type TextMsg struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	RequestID string `json:"requestId,omitempty"`
}

type TextAcceptedMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	RequestID string `json:"requestId"`
}

// ---- 出站消息 ----

type HelloMsg struct {
	Type            string `json:"type"`
	AgentVersion    string `json:"agentVersion"`
	ClaudeVersion   string `json:"claudeVersion"`
	ProtocolVersion string `json:"protocolVersion"`
}

type SessionInfo struct {
	SessionID   string   `json:"sessionId"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Owner       string   `json:"owner"`
	Subscribers []string `json:"subscribers"`
	CreatedAt   int64    `json:"createdAt"`
	Cwd         string   `json:"cwd"`
	Provider    string   `json:"provider"`
	Model       string   `json:"model,omitempty"`
	Permission  string   `json:"permissionMode"`
}

type SessionListMsg struct {
	Type     string        `json:"type"`
	Sessions []SessionInfo `json:"sessions"`
}

type ProjectInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Permission string `json:"permission,omitempty"`
}

type ProjectListMsg struct {
	Type     string        `json:"type"`
	Projects []ProjectInfo `json:"projects"`
}

type ProviderPermission struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Dangerous   bool   `json:"dangerous,omitempty"`
	Mutable     bool   `json:"mutable"`
}

type ProviderInfo struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Available         bool                 `json:"available"`
	UnavailableReason string               `json:"unavailableReason,omitempty"`
	Permissions       []ProviderPermission `json:"permissions"`
	Models            []string             `json:"models,omitempty"`
}

type ProviderListMsg struct {
	Type      string         `json:"type"`
	Providers []ProviderInfo `json:"providers"`
}

type TemplateInfo struct {
	TemplateID string `json:"templateId,omitempty" yaml:"-"`
	Label      string `json:"label" yaml:"label"`
	Prompt     string `json:"prompt" yaml:"prompt"`
}

type TemplateListMsg struct {
	Type      string         `json:"type"`
	Templates []TemplateInfo `json:"templates"`
}

type SessionCreatedMsg struct {
	Type           string `json:"type"`
	SessionID      string `json:"sessionId"`
	Name           string `json:"name"`
	Cwd            string `json:"cwd"`
	Provider       string `json:"provider"`
	Model          string `json:"model,omitempty"`
	PermissionMode string `json:"permissionMode"`
	RequestID      string `json:"requestId,omitempty"`
}

type SessionStoppedMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
}

type PermissionChangedMsg struct {
	Type           string `json:"type"`
	SessionID      string `json:"sessionId"`
	PermissionMode string `json:"permissionMode"`
	Pending        bool   `json:"pending"`
}

type TokenMsg struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ThinkingMsg struct {
	Type string `json:"type"`
}
type DoneMsg struct {
	Type string `json:"type"`
}

type PongMsg struct {
	Type string `json:"type"`
}

type HealthMsg struct {
	Type        string `json:"type"`
	SessionID   string `json:"sessionId"`
	State       string `json:"state"`
	IdleSeconds int64  `json:"idleSeconds"`
}

type QueuedMsg struct {
	Type     string `json:"type"`
	MsgID    string `json:"msgId"`
	Position int    `json:"position"`
}

type DequeuedMsg struct {
	Type  string `json:"type"`
	MsgID string `json:"msgId"`
}

type HistoryMsg struct {
	Type      string            `json:"type"`
	SessionID string            `json:"sessionId"`
	Messages  []json.RawMessage `json:"messages"`
}

type ToolUseMsg struct {
	Type  string `json:"type"`
	Tool  string `json:"tool"`
	Input string `json:"input"`
}

type ErrorMsg struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewError 构造一条 error 消息。
func NewError(code, msg string) ErrorMsg {
	return ErrorMsg{Type: TypeError, Code: code, Message: msg}
}
