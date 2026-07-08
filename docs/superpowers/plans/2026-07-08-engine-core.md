# 引擎核心 (Engine Core) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现 Claude Phone 的可复用引擎核心（协议 + 会话管理 + claude 子进程驱动 + WebSocket 服务 + tsnet 网络），并用无头入口 `cmd/mac-agent` 跑通，可独立测试。

**Architecture:** 分层：`pkg/protocol`（三端共享 JSON 协议）→ `pkg/session`（会话与订阅者管理，claude 子进程驱动）→ `pkg/engine`（WebSocket 服务 + tsnet 接入 + 鉴权路由 + 扇出）→ `cmd/mac-agent`（无头入口）。引擎完全不感知任何 GUI，是后续 Mac 桌面 App（Plan 2）和手机端共同依赖的底座。

**Tech Stack:** Go 1.26；`github.com/gorilla/websocket`（WS）；`tailscale.com/tsnet`（网络）；Go 标准库 `testing`（测试）；`os/exec`（claude 子进程）。

**依赖前置:** 本计划无外部前置，从当前脚手架（仅 `pkg/androidlib`）起步。这是 Plan 2（Mac 桌面 GUI）的前置。

**范围边界（YAGNI）:** 本计划只做引擎核心 + 无头入口。不做：GUI、systray、adminproto 管理协议、tsnet 真实跨网络端到端（用本地 loopback listener 测试 WS 逻辑，tsnet 仅做最小接入封装）。语音（voice）消息按普通文本处理。

---

## 文件结构

| 文件 | 职责 | 创建/修改 |
|---|---|---|
| `go.mod` / `go.sum` | 加 gorilla/websocket、tsnet 依赖 | 修改 |
| `pkg/protocol/messages.go` | 消息类型常量、Envelope、错误码、编解码 | 创建 |
| `pkg/protocol/messages_test.go` | 协议编解码测试 | 创建 |
| `pkg/session/session.go` | 单个会话：订阅者集合、消息队列、状态 | 创建 |
| `pkg/session/claude.go` | claude 子进程驱动（stream-json translate 层） | 创建 |
| `pkg/session/manager.go` | 会话管理：创建/查询/列表/并发上限 | 创建 |
| `pkg/session/*_test.go` | 会话与子进程测试（用 fake claude 脚本） | 创建 |
| `pkg/engine/engine.go` | 引擎装配：持有 Manager + 配置 | 创建 |
| `pkg/engine/wsserver.go` | WebSocket 服务：鉴权、路由、扇出 | 创建 |
| `pkg/engine/wsserver_test.go` | WS 端到端测试（Go WS 客户端） | 创建 |
| `pkg/engine/tsnet.go` | tsnet 最小接入封装（可选启用） | 创建 |
| `pkg/engine/config.go` | 引擎配置（端口、并发上限、compatibleRange 等） | 创建 |
| `cmd/mac-agent/main.go` | 无头入口：起引擎 + 优雅退出 | 创建 |
| `testdata/fake-claude.sh` | 模拟 claude CLI 的测试脚本 | 创建 |

---

### Task 1: 添加依赖并锁定

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: 添加 gorilla/websocket 与 tsnet 依赖**

Run:
```bash
go get github.com/gorilla/websocket@v1.5.3
go get tailscale.com/tsnet@latest
```
Expected: `go.mod` 出现两条 `require`，`go.sum` 更新。

- [ ] **Step 2: 整理依赖**

Run: `go mod tidy`
Expected: 无错误，间接依赖补齐。

- [ ] **Step 3: 验证可编译**

Run: `go build ./...`
Expected: 无错误（此时只有 `pkg/androidlib`，应通过）。

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: 添加 gorilla/websocket + tsnet 依赖"
```

---

### Task 2: 协议消息定义 (`pkg/protocol`)

**Files:**
- Create: `pkg/protocol/messages.go`
- Test: `pkg/protocol/messages_test.go`

对应 README §5.1 / §5.2 / §5.3。核心思路：所有 WS 消息是 JSON 对象，含 `type` 字段。用一个通用 `Envelope`（`type` + 原始 `json.RawMessage`）先解出类型，再按需解具体结构。出站消息用具体 struct 直接 marshal。

- [ ] **Step 1: 写失败的测试**

`pkg/protocol/messages_test.go`:
```go
package protocol

import (
	"encoding/json"
	"testing"
)

func TestParseEnvelope_Auth(t *testing.T) {
	raw := []byte(`{"type":"auth","deviceToken":"dt_abc","deviceName":"Pixel 8"}`)
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("ParseEnvelope error: %v", err)
	}
	if env.Type != TypeAuth {
		t.Fatalf("got type %q, want %q", env.Type, TypeAuth)
	}
	var a AuthMsg
	if err := json.Unmarshal(env.Raw, &a); err != nil {
		t.Fatalf("unmarshal AuthMsg: %v", err)
	}
	if a.DeviceToken != "dt_abc" || a.DeviceName != "Pixel 8" {
		t.Fatalf("bad AuthMsg: %+v", a)
	}
}

func TestParseControl_Action(t *testing.T) {
	raw := []byte(`{"type":"control","action":"create_session","name":"车险联调","workingDir":"/p","permissionMode":"bypassPermissions"}`)
	env, err := ParseEnvelope(raw)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if env.Type != TypeControl {
		t.Fatalf("type=%q", env.Type)
	}
	var c ControlMsg
	if err := json.Unmarshal(env.Raw, &c); err != nil {
		t.Fatalf("unmarshal control: %v", err)
	}
	if c.Action != ActionCreateSession || c.Name != "车险联调" || c.WorkingDir != "/p" {
		t.Fatalf("bad control: %+v", c)
	}
}

func TestErrorMsg_Marshal(t *testing.T) {
	b, err := json.Marshal(NewError(CodeSessionNotFound, "会话不存在"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"type":"error","code":"SESSION_NOT_FOUND","message":"会话不存在"}`
	if string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./pkg/protocol/ -run Test -v`
Expected: FAIL（`undefined: ParseEnvelope` 等）。

- [ ] **Step 3: 写最小实现**

`pkg/protocol/messages.go`:
```go
// Package protocol 定义三端共享的 WebSocket JSON 消息与错误码。
// 对应 README §5.1(手机→Mac) / §5.2(Mac→手机) / §5.3(错误码)。
package protocol

import "encoding/json"

const ProtocolVersion = "1"

// 消息 type 常量（手机→Mac 与 Mac→手机 合并列出）。
const (
	TypeAuth               = "auth"
	TypeControl            = "control"
	TypeText               = "text"
	TypeVoice              = "voice"
	TypePermissionResponse = "permission_response"
	TypePermissionRule     = "permission_rule"

	TypeHello           = "hello"
	TypeProjectList     = "project_list"
	TypeSessionList     = "session_list"
	TypeSessionCreated  = "session_created"
	TypeSessionSelected = "session_selected"
	TypeSessionStopped  = "session_stopped"
	TypeThinking        = "thinking"
	TypeToken           = "token"
	TypeToolUse         = "tool_use"
	TypeDone            = "done"
	TypeError           = "error"
	TypeQueued          = "queued"
	TypeDequeued        = "dequeued"
	TypePong            = "pong"
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
	ActionCancel        = "cancel"
	ActionLoadHistory   = "load_history"
	ActionPing          = "ping"
	ActionForceKill     = "force_kill"
	ActionWaitLonger    = "wait_longer"
)

// 错误码，对应 README §5.3。
const (
	CodeSessionNotFound     = "SESSION_NOT_FOUND"
	CodeSessionNotOwner     = "SESSION_NOT_OWNER"
	CodeSessionLimitReached = "SESSION_LIMIT_REACHED"
	CodeDeviceNotAuthorized = "DEVICE_NOT_AUTHORIZED"
	CodeClaudeNotFound      = "CLAUDE_NOT_FOUND"
	CodeClaudeVersionMismatch = "CLAUDE_VERSION_MISMATCH"
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
	return Envelope{Type: head.Type, Raw: append(json.RawMessage(nil), b...)}, nil
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
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
	BeforeMsgID    string `json:"beforeMsgId,omitempty"`
}

type TextMsg struct {
	Type    string `json:"type"`
	Content string `json:"content"`
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
}

type SessionListMsg struct {
	Type     string        `json:"type"`
	Sessions []SessionInfo `json:"sessions"`
}

type SessionCreatedMsg struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Cwd       string `json:"cwd"`
}

type TokenMsg struct {
	Type    string `json:"type"`
	Content string `json:"content"`
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./pkg/protocol/ -v`
Expected: PASS（3 个测试全过）。

- [ ] **Step 5: Commit**

```bash
git add pkg/protocol/
git commit -m "feat(protocol): 三端共享 WS 消息类型 + 错误码 + 编解码"
```

---

### Task 3: 会话对象 (`pkg/session/session.go`)

**Files:**
- Create: `pkg/session/session.go`
- Test: `pkg/session/session_test.go`

单个 `Session`：持有 owner、subscribers（设备 ID 集合）、per-session FIFO 消息队列（README §4.4）、状态。`Session` 提供订阅者增删与"向所有订阅者广播出站消息"的扇出接口（扇出的实际发送通过回调，便于测试，不耦合 WS）。

- [ ] **Step 1: 写失败的测试**

`pkg/session/session_test.go`:
```go
package session

import (
	"sync"
	"testing"
)

func TestSubscribeUnsubscribe(t *testing.T) {
	s := NewSession("sess1", "车险联调", "/p", "device-A")
	if s.Owner != "device-A" {
		t.Fatalf("owner=%q", s.Owner)
	}
	s.Subscribe("device-B")
	subs := s.Subscribers()
	if len(subs) != 2 {
		t.Fatalf("want 2 subs, got %v", subs)
	}
	s.Unsubscribe("device-B")
	if len(s.Subscribers()) != 1 {
		t.Fatalf("unsubscribe failed: %v", s.Subscribers())
	}
}

func TestBroadcastFanOut(t *testing.T) {
	s := NewSession("sess1", "n", "/p", "device-A")
	s.Subscribe("device-B")

	var mu sync.Mutex
	got := map[string]int{}
	s.SetSender(func(deviceID string, payload []byte) {
		mu.Lock()
		got[deviceID]++
		mu.Unlock()
	})
	s.Broadcast([]byte(`{"type":"token","content":"hi"}`))

	if got["device-A"] != 1 || got["device-B"] != 1 {
		t.Fatalf("fan-out wrong: %v", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./pkg/session/ -run TestSubscribe -v`
Expected: FAIL（`undefined: NewSession`）。

- [ ] **Step 3: 写最小实现**

`pkg/session/session.go`:
```go
// Package session 管理 Claude Phone 的会话生命周期、订阅者扇出与 claude 子进程驱动。
package session

import "sync"

// SenderFunc 把 payload 发给某个设备（由 WS 层注入，测试可替换）。
type SenderFunc func(deviceID string, payload []byte)

// Session 表示一个 claude 会话及其订阅者。
type Session struct {
	ID         string
	Name       string
	Cwd        string
	Owner      string
	Status     string // active | dormant | stopped
	CreatedAt  int64

	mu     sync.RWMutex
	subs   map[string]struct{}
	sender SenderFunc
}

// NewSession 创建会话，owner 自动成为首个订阅者。
func NewSession(id, name, cwd, owner string) *Session {
	return &Session{
		ID:     id,
		Name:   name,
		Cwd:    cwd,
		Owner:  owner,
		Status: "active",
		subs:   map[string]struct{}{owner: {}},
	}
}

// SetSender 注入发送回调。
func (s *Session) SetSender(fn SenderFunc) {
	s.mu.Lock()
	s.sender = fn
	s.mu.Unlock()
}

func (s *Session) Subscribe(deviceID string) {
	s.mu.Lock()
	s.subs[deviceID] = struct{}{}
	s.mu.Unlock()
}

func (s *Session) Unsubscribe(deviceID string) {
	s.mu.Lock()
	delete(s.subs, deviceID)
	s.mu.Unlock()
}

// Subscribers 返回当前订阅者设备 ID 列表（快照）。
func (s *Session) Subscribers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.subs))
	for id := range s.subs {
		out = append(out, id)
	}
	return out
}

// Broadcast 把 payload 扇出给所有订阅者。
func (s *Session) Broadcast(payload []byte) {
	s.mu.RLock()
	sender := s.sender
	ids := make([]string, 0, len(s.subs))
	for id := range s.subs {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	if sender == nil {
		return
	}
	for _, id := range ids {
		sender(id, payload)
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./pkg/session/ -run 'TestSubscribe|TestBroadcast' -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add pkg/session/session.go pkg/session/session_test.go
git commit -m "feat(session): 会话对象 + 订阅者扇出"
```

---

### Task 4: claude 子进程驱动 (`pkg/session/claude.go`)

**Files:**
- Create: `pkg/session/claude.go`
- Create: `testdata/fake-claude.sh`
- Test: `pkg/session/claude_test.go`

对应 README §4.5。`ClaudeProc` 用 `os/exec` 启动 claude（`--print --input-format stream-json --output-format stream-json ...`），把用户文本写进 stdin（封成 stream-json 行），逐行读 stdout 的 JSON，转成 `protocol` 出站消息，通过回调交给上层扇出。测试用 `fake-claude.sh` 模拟：读一行 stdin，输出几行 stream-json。

- [ ] **Step 1: 写 fake claude 脚本**

`testdata/fake-claude.sh`:
```bash
#!/usr/bin/env bash
# 模拟 claude --print --output-format stream-json:
# 读取一行 stdin(JSON), 输出 thinking → 两个 token → done 的 stream-json 行。
read -r _line
printf '{"type":"thinking"}\n'
printf '{"type":"token","content":"hello "}\n'
printf '{"type":"token","content":"world"}\n'
printf '{"type":"done"}\n'
```
然后：
```bash
chmod +x testdata/fake-claude.sh
```

- [ ] **Step 2: 写失败的测试**

`pkg/session/claude_test.go`:
```go
package session

import (
	"sync"
	"testing"
	"time"
)

func TestClaudeProc_StreamsTokens(t *testing.T) {
	proc := NewClaudeProc(ClaudeConfig{
		Bin:        "../../testdata/fake-claude.sh",
		Cwd:        ".",
		SessionID:  "sess1",
		Permission: "bypassPermissions",
	})

	var mu sync.Mutex
	var lines []string
	proc.OnOutput(func(payload []byte) {
		mu.Lock()
		lines = append(lines, string(payload))
		mu.Unlock()
	})

	if err := proc.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := proc.Send("检查并发"); err != nil {
		t.Fatalf("send: %v", err)
	}

	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := len(lines)
		mu.Unlock()
		if n >= 4 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout, got %d lines: %v", n, lines)
		case <-time.After(20 * time.Millisecond):
		}
	}
	_ = proc.Stop()

	if lines[0] != `{"type":"thinking"}` {
		t.Fatalf("first line = %s", lines[0])
	}
	if lines[len(lines)-1] != `{"type":"done"}` {
		t.Fatalf("last line = %s", lines[len(lines)-1])
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./pkg/session/ -run TestClaudeProc -v`
Expected: FAIL（`undefined: NewClaudeProc`）。

- [ ] **Step 4: 写最小实现**

`pkg/session/claude.go`:
```go
package session

import (
	"bufio"
	"encoding/json"
	"io"
	"os/exec"
	"sync"
)

// ClaudeConfig 是启动 claude 子进程所需的参数。
type ClaudeConfig struct {
	Bin        string   // claude 可执行文件路径（默认 "claude"）
	Cwd        string   // 工作目录
	SessionID  string   // 固定 session-id，支持 --resume
	Permission string   // bypassPermissions | acceptEdits | default
	AddDirs    []string // 额外 --add-dir
}

// OutputFunc 接收一行 claude stdout 的原始 JSON。
type OutputFunc func(payload []byte)

// ClaudeProc 驱动单个 claude 子进程（translate 层）。
type ClaudeProc struct {
	cfg    ClaudeConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	mu     sync.Mutex
	onOut  OutputFunc
}

func NewClaudeProc(cfg ClaudeConfig) *ClaudeProc {
	if cfg.Bin == "" {
		cfg.Bin = "claude"
	}
	return &ClaudeProc{cfg: cfg}
}

func (p *ClaudeProc) OnOutput(fn OutputFunc) {
	p.mu.Lock()
	p.onOut = fn
	p.mu.Unlock()
}

// buildArgs 组装 claude CLI 参数（README §4.5）。
func (p *ClaudeProc) buildArgs() []string {
	args := []string{
		"--print",
		"--session-id", p.cfg.SessionID,
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--permission-mode", p.cfg.Permission,
		"--replay-user-messages",
	}
	for _, d := range p.cfg.AddDirs {
		args = append(args, "--add-dir", d)
	}
	return args
}

func (p *ClaudeProc) Start() error {
	cmd := exec.Command(p.cfg.Bin, p.buildArgs()...)
	cmd.Dir = p.cfg.Cwd
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	p.cmd, p.stdin, p.stdout = cmd, stdin, stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	go p.readLoop()
	return nil
}

func (p *ClaudeProc) readLoop() {
	sc := bufio.NewScanner(p.stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := append([]byte(nil), sc.Bytes()...)
		p.mu.Lock()
		fn := p.onOut
		p.mu.Unlock()
		if fn != nil && len(line) > 0 {
			fn(line)
		}
	}
}

// Send 把用户文本封成 stream-json 一行写入 stdin。
func (p *ClaudeProc) Send(text string) error {
	msg := map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": text,
		},
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = p.stdin.Write(b)
	return err
}

// Stop 关闭 stdin 并等待进程退出。
func (p *ClaudeProc) Stop() error {
	if p.stdin != nil {
		_ = p.stdin.Close()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		return p.cmd.Wait()
	}
	return nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./pkg/session/ -run TestClaudeProc -v`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add pkg/session/claude.go pkg/session/claude_test.go testdata/fake-claude.sh
git commit -m "feat(session): claude 子进程 translate 层 (stream-json stdin/stdout)"
```

---

### Task 5: 会话管理器 (`pkg/session/manager.go`)

**Files:**
- Create: `pkg/session/manager.go`
- Test: `pkg/session/manager_test.go`

`Manager` 持有 `map[sessionID]*Session`，负责创建（带并发上限 README §4.5）、查询、列表。创建时校验上限，超限返回哨兵错误 `ErrSessionLimit`（上层翻成 `SESSION_LIMIT_REACHED`）。会话 ID 由注入的 `IDFunc` 生成（测试可控，生产用 UUID）。

- [ ] **Step 1: 写失败的测试**

`pkg/session/manager_test.go`:
```go
package session

import (
	"errors"
	"testing"
)

func newTestManager(limit int) *Manager {
	n := 0
	return NewManager(ManagerConfig{
		MaxConcurrent: limit,
		IDFunc: func() string {
			n++
			return "sess-" + string(rune('0'+n))
		},
	})
}

func TestManager_CreateAndGet(t *testing.T) {
	m := newTestManager(5)
	s, err := m.Create("车险", "/p", "bypassPermissions", "device-A")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, ok := m.Get(s.ID)
	if !ok || got.Name != "车险" {
		t.Fatalf("get failed: %v %v", got, ok)
	}
	if len(m.List()) != 1 {
		t.Fatalf("list len = %d", len(m.List()))
	}
}

func TestManager_ConcurrencyLimit(t *testing.T) {
	m := newTestManager(2)
	_, _ = m.Create("a", "/p", "default", "d")
	_, _ = m.Create("b", "/p", "default", "d")
	_, err := m.Create("c", "/p", "default", "d")
	if !errors.Is(err, ErrSessionLimit) {
		t.Fatalf("want ErrSessionLimit, got %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./pkg/session/ -run TestManager -v`
Expected: FAIL（`undefined: NewManager`）。

- [ ] **Step 3: 写最小实现**

`pkg/session/manager.go`:
```go
package session

import (
	"errors"
	"sync"
	"time"
)

// ErrSessionLimit 表示并发会话数达到上限（对应 SESSION_LIMIT_REACHED）。
var ErrSessionLimit = errors.New("session limit reached")

// ManagerConfig 配置会话管理器。
type ManagerConfig struct {
	MaxConcurrent int           // 并发会话上限（README §4.5，默认 5）
	IDFunc        func() string // 会话 ID 生成器
	Now           func() int64  // 时间源（测试可注入），默认 time.Now().Unix()
}

// Manager 管理所有活跃会话。
type Manager struct {
	cfg  ManagerConfig
	mu   sync.RWMutex
	byID map[string]*Session
}

func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.Now == nil {
		cfg.Now = func() int64 { return time.Now().Unix() }
	}
	return &Manager{cfg: cfg, byID: map[string]*Session{}}
}

// Create 新建会话，超过并发上限返回 ErrSessionLimit。
func (m *Manager) Create(name, cwd, permission, owner string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.byID) >= m.cfg.MaxConcurrent {
		return nil, ErrSessionLimit
	}
	id := m.cfg.IDFunc()
	s := NewSession(id, name, cwd, owner)
	s.CreatedAt = m.cfg.Now()
	m.byID[id] = s
	return s, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byID[id]
	return s, ok
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	delete(m.byID, id)
	m.mu.Unlock()
}

// List 返回所有会话（快照）。
func (m *Manager) List() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Session, 0, len(m.byID))
	for _, s := range m.byID {
		out = append(out, s)
	}
	return out
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./pkg/session/ -v`
Expected: PASS（session/claude/manager 全部测试）。

- [ ] **Step 5: Commit**

```bash
git add pkg/session/manager.go pkg/session/manager_test.go
git commit -m "feat(session): 会话管理器 + 并发上限"
```

---

### Task 6: 引擎配置 (`pkg/engine/config.go`)

**Files:**
- Create: `pkg/engine/config.go`
- Test: `pkg/engine/config_test.go`

`Config` 含端口、`MaxConcurrentSessions`、`compatibleRange`、claude bin 路径、tsnet 状态目录、AgentVersion。提供 `CheckClaudeVersion(ver string)`：解析 semver，判断是否落在 compatibleRange 内。

> **注意（实测）:** 本机 `claude --version` = 2.1.186，而 README 示例 range 是 `>=1.0.0,<2.0.0`——会拒绝启动。本计划默认 range 设为 `>=1.0.0,<3.0.0` 以接受当前 2.x；range 可配置。

- [ ] **Step 1: 写失败的测试**

`pkg/engine/config_test.go`:
```go
package engine

import "testing"

func TestCheckClaudeVersion(t *testing.T) {
	c := DefaultConfig()
	if !c.CheckClaudeVersion("2.1.186") {
		t.Fatalf("2.1.186 should be compatible with default range %s", c.CompatibleRange)
	}
	if c.CheckClaudeVersion("3.0.0") {
		t.Fatalf("3.0.0 should be incompatible")
	}
	if c.CheckClaudeVersion("0.9.0") {
		t.Fatalf("0.9.0 should be incompatible")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./pkg/engine/ -run TestCheckClaudeVersion -v`
Expected: FAIL（`undefined: DefaultConfig`）。

- [ ] **Step 3: 写最小实现**

`pkg/engine/config.go`:
```go
// Package engine 装配 Claude Phone 引擎：会话管理 + WebSocket 服务 + tsnet 接入。
package engine

import (
	"strconv"
	"strings"
)

const AgentVersion = "0.1.0"

// Config 是引擎运行配置。
type Config struct {
	Port                  int    // WS 监听端口（README: 9876）
	MaxConcurrentSessions int    // 并发会话上限
	CompatibleRange       string // 形如 ">=1.0.0,<3.0.0"
	ClaudeBin             string // claude 可执行文件路径
	TsnetDir              string // tsnet 状态持久化目录
	Hostname              string // tsnet hostname
}

// DefaultConfig 返回带默认值的配置。
func DefaultConfig() Config {
	return Config{
		Port:                  9876,
		MaxConcurrentSessions: 5,
		CompatibleRange:       ">=1.0.0,<3.0.0",
		ClaudeBin:             "claude",
		Hostname:              "claude-mac",
	}
}

// semver 三段解析，非法段按 0 处理。
func parseSemver(v string) [3]int {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	// 去掉可能的构建/预发布后缀
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func cmpSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// CheckClaudeVersion 判断 ver 是否落在 CompatibleRange 内。
// 只支持 ">=X,<Y" 两段式（够用即可，YAGNI）。
func (c Config) CheckClaudeVersion(ver string) bool {
	v := parseSemver(ver)
	for _, clause := range strings.Split(c.CompatibleRange, ",") {
		clause = strings.TrimSpace(clause)
		switch {
		case strings.HasPrefix(clause, ">="):
			if cmpSemver(v, parseSemver(clause[2:])) < 0 {
				return false
			}
		case strings.HasPrefix(clause, "<"):
			if cmpSemver(v, parseSemver(clause[1:])) >= 0 {
				return false
			}
		}
	}
	return true
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./pkg/engine/ -run TestCheckClaudeVersion -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add pkg/engine/config.go pkg/engine/config_test.go
git commit -m "feat(engine): 引擎配置 + claude 版本兼容检查"
```

---

### Task 7: WebSocket 服务与鉴权路由 (`pkg/engine/wsserver.go` + `engine.go`)

**Files:**
- Create: `pkg/engine/engine.go`
- Create: `pkg/engine/wsserver.go`
- Test: `pkg/engine/wsserver_test.go`

`Engine` 持有 `Manager` + `Config` + 设备连接注册表（`deviceID -> *websocket.Conn`）。WS 处理流程：升级连接 → 等第一条 `auth` → 校验 device token（本计划用简单内存白名单，真实吊销/配对留给后续）→ 回 `hello` → 循环读消息按 `type`/`action` 路由。`create_session` 超限回 `SESSION_LIMIT_REACHED`；`text` 转发给会话的 claude 子进程；claude 输出扇出给订阅者。设备发送通过 `Session.SetSender` 注入的回调（按 deviceID 找 conn 写）。

测试用真实 `httptest.Server` + gorilla WS 客户端，走真实 WS 握手与消息收发（不是 mock）。

- [ ] **Step 1: 写失败的测试**

`pkg/engine/wsserver_test.go`:
```go
package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func dial(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return c
}

func readJSON(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, b, err := c.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	return m
}

func newTestEngine() *Engine {
	cfg := DefaultConfig()
	cfg.ClaudeBin = "../../testdata/fake-claude.sh"
	e := New(cfg)
	e.AuthorizeDevice("dt_test") // 内存白名单
	return e
}

func TestWS_AuthThenHello(t *testing.T) {
	e := newTestEngine()
	srv := httptest.NewServer(http.HandlerFunc(e.HandleWS))
	defer srv.Close()

	c := dial(t, srv)
	defer c.Close()
	c.WriteJSON(map[string]string{"type": "auth", "deviceToken": "dt_test", "deviceName": "Pixel"})

	m := readJSON(t, c)
	if m["type"] != "hello" {
		t.Fatalf("want hello, got %v", m)
	}
}

func TestWS_CreateSessionAndStream(t *testing.T) {
	e := newTestEngine()
	srv := httptest.NewServer(http.HandlerFunc(e.HandleWS))
	defer srv.Close()

	c := dial(t, srv)
	defer c.Close()
	c.WriteJSON(map[string]string{"type": "auth", "deviceToken": "dt_test", "deviceName": "Pixel"})
	readJSON(t, c) // hello

	c.WriteJSON(map[string]string{
		"type": "control", "action": "create_session",
		"name": "车险", "workingDir": ".", "permissionMode": "bypassPermissions",
	})
	created := readJSON(t, c)
	if created["type"] != "session_created" {
		t.Fatalf("want session_created, got %v", created)
	}

	c.WriteJSON(map[string]string{"type": "text", "content": "hi"})

	// 期望依次收到 thinking → token → token → done
	sawDone := false
	for i := 0; i < 6 && !sawDone; i++ {
		m := readJSON(t, c)
		if m["type"] == "done" {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatalf("did not receive done")
	}
}

func TestWS_BadTokenRejected(t *testing.T) {
	e := newTestEngine()
	srv := httptest.NewServer(http.HandlerFunc(e.HandleWS))
	defer srv.Close()

	c := dial(t, srv)
	defer c.Close()
	c.WriteJSON(map[string]string{"type": "auth", "deviceToken": "WRONG", "deviceName": "x"})
	m := readJSON(t, c)
	if m["type"] != "error" || m["code"] != "DEVICE_NOT_AUTHORIZED" {
		t.Fatalf("want DEVICE_NOT_AUTHORIZED, got %v", m)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./pkg/engine/ -run TestWS -v`
Expected: FAIL（`undefined: New` / `HandleWS`）。

- [ ] **Step 3: 写 `engine.go`**

`pkg/engine/engine.go`:
```go
package engine

import (
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/session"
)

// Engine 装配会话管理与设备连接注册表。
type Engine struct {
	cfg Config
	mgr *session.Manager

	mu       sync.RWMutex
	authced  map[string]struct{} // 授权的 device token 白名单
	conns    map[string]connWriter // deviceID -> 写接口
	deviceSess map[string]string   // deviceID -> 当前活跃 sessionID
}

// connWriter 抽象一个设备连接的写能力（便于测试）。
type connWriter interface {
	WriteMessage(messageType int, data []byte) error
}

func New(cfg Config) *Engine {
	n := 0
	mgr := session.NewManager(session.ManagerConfig{
		MaxConcurrent: cfg.MaxConcurrentSessions,
		IDFunc: func() string {
			n++
			return "sess-" + itoa(n)
		},
	})
	return &Engine{
		cfg:        cfg,
		mgr:        mgr,
		authced:    map[string]struct{}{},
		conns:      map[string]connWriter{},
		deviceSess: map[string]string{},
	}
}

// AuthorizeDevice 把一个 device token 加入白名单（配对成功后调用）。
func (e *Engine) AuthorizeDevice(token string) {
	e.mu.Lock()
	e.authced[token] = struct{}{}
	e.mu.Unlock()
}

func (e *Engine) isAuthorized(token string) bool {
	e.mu.RLock()
	_, ok := e.authced[token]
	e.mu.RUnlock()
	return ok
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
```

- [ ] **Step 4: 写 `wsserver.go`**

`pkg/engine/wsserver.go`:
```go
package engine

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"github.com/yang-bin-free/claude-phone/pkg/session"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 本地/受信网络，放行
}

// HandleWS 是 WebSocket 入口 handler。
func (e *Engine) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 1) 第一条必须是 auth
	_, raw, err := conn.ReadMessage()
	if err != nil {
		return
	}
	env, err := protocol.ParseEnvelope(raw)
	if err != nil || env.Type != protocol.TypeAuth {
		writeJSON(conn, protocol.NewError(protocol.CodeDeviceNotAuthorized, "缺少认证"))
		return
	}
	var auth protocol.AuthMsg
	_ = json.Unmarshal(env.Raw, &auth)
	if !e.isAuthorized(auth.DeviceToken) {
		writeJSON(conn, protocol.NewError(protocol.CodeDeviceNotAuthorized, "device token 无效"))
		return
	}
	deviceID := auth.DeviceToken // 本计划用 token 作 deviceID，配对体系后续细化

	// 注册连接
	e.mu.Lock()
	e.conns[deviceID] = conn
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.conns, deviceID)
		e.mu.Unlock()
	}()

	// 2) 回 hello
	claudeVer := "unknown"
	writeJSON(conn, protocol.HelloMsg{
		Type: protocol.TypeHello, AgentVersion: AgentVersion,
		ClaudeVersion: claudeVer, ProtocolVersion: protocol.ProtocolVersion,
	})

	// 3) 消息循环
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		env, err := protocol.ParseEnvelope(raw)
		if err != nil {
			continue
		}
		switch env.Type {
		case protocol.TypeControl:
			e.handleControl(deviceID, env.Raw)
		case protocol.TypeText, protocol.TypeVoice:
			var tm protocol.TextMsg
			_ = json.Unmarshal(env.Raw, &tm)
			e.handleText(deviceID, tm.Content)
		}
	}
}

func (e *Engine) handleControl(deviceID string, raw []byte) {
	var c protocol.ControlMsg
	_ = json.Unmarshal(raw, &c)
	switch c.Action {
	case protocol.ActionCreateSession:
		s, err := e.mgr.Create(c.Name, c.WorkingDir, c.PermissionMode, deviceID)
		if err == session.ErrSessionLimit {
			e.sendTo(deviceID, protocol.NewError(protocol.CodeSessionLimitReached, "并发会话已达上限"))
			return
		}
		// 注入扇出发送器
		s.SetSender(func(id string, payload []byte) { e.sendRaw(id, payload) })
		// 启动 claude 子进程
		proc := session.NewClaudeProc(session.ClaudeConfig{
			Bin: e.cfg.ClaudeBin, Cwd: c.WorkingDir,
			SessionID: s.ID, Permission: c.PermissionMode,
		})
		proc.OnOutput(func(payload []byte) { s.Broadcast(payload) })
		_ = proc.Start()
		e.attachProc(s.ID, proc)
		e.setActive(deviceID, s.ID)
		e.sendTo(deviceID, protocol.SessionCreatedMsg{
			Type: protocol.TypeSessionCreated, SessionID: s.ID, Name: s.Name, Cwd: s.Cwd,
		})
	case protocol.ActionListSessions:
		e.sendTo(deviceID, e.buildSessionList())
	case protocol.ActionPing:
		e.sendRaw(deviceID, []byte(`{"type":"pong"}`))
	}
}

func (e *Engine) handleText(deviceID, content string) {
	e.mu.RLock()
	sid := e.deviceSess[deviceID]
	e.mu.RUnlock()
	proc := e.getProc(sid)
	if proc == nil {
		e.sendTo(deviceID, protocol.NewError(protocol.CodeSessionNotFound, "无活跃会话"))
		return
	}
	_ = proc.Send(content)
}

func (e *Engine) buildSessionList() protocol.SessionListMsg {
	var infos []protocol.SessionInfo
	for _, s := range e.mgr.List() {
		infos = append(infos, protocol.SessionInfo{
			SessionID: s.ID, Name: s.Name, Status: s.Status,
			Owner: s.Owner, Subscribers: s.Subscribers(), CreatedAt: s.CreatedAt,
		})
	}
	return protocol.SessionListMsg{Type: protocol.TypeSessionList, Sessions: infos}
}

// ---- 发送辅助 ----

func writeJSON(conn *websocket.Conn, v any) {
	b, _ := json.Marshal(v)
	_ = conn.WriteMessage(websocket.TextMessage, b)
}

func (e *Engine) sendTo(deviceID string, v any) {
	b, _ := json.Marshal(v)
	e.sendRaw(deviceID, b)
}

func (e *Engine) sendRaw(deviceID string, payload []byte) {
	e.mu.RLock()
	w := e.conns[deviceID]
	e.mu.RUnlock()
	if w != nil {
		_ = w.WriteMessage(websocket.TextMessage, payload)
	}
}

func (e *Engine) setActive(deviceID, sessionID string) {
	e.mu.Lock()
	e.deviceSess[deviceID] = sessionID
	e.mu.Unlock()
}
```

- [ ] **Step 5: 补 `engine.go` 的进程注册表（attachProc/getProc）**

在 `pkg/engine/engine.go` 的 `Engine` 结构体加字段并新增方法：
```go
// 在 Engine struct 内新增：
//   procs map[string]*session.ClaudeProc

// 在 New() 里初始化：
//   procs: map[string]*session.ClaudeProc{},

// 新增方法：
func (e *Engine) attachProc(sessionID string, p *session.ClaudeProc) {
	e.mu.Lock()
	e.procs[sessionID] = p
	e.mu.Unlock()
}

func (e *Engine) getProc(sessionID string) *session.ClaudeProc {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.procs[sessionID]
}
```
完整 `Engine` struct 字段应为：`cfg`、`mgr`、`authced`、`conns`、`deviceSess`、`procs`。

- [ ] **Step 6: 运行测试确认通过**

Run: `go test ./pkg/engine/ -run TestWS -v`
Expected: PASS（AuthThenHello、CreateSessionAndStream、BadTokenRejected 全过）。

- [ ] **Step 7: 全量测试**

Run: `go test ./... -v`
Expected: PASS（protocol / session / engine 全部）。

- [ ] **Step 8: Commit**

```bash
git add pkg/engine/engine.go pkg/engine/wsserver.go pkg/engine/wsserver_test.go
git commit -m "feat(engine): WebSocket 服务 + 鉴权路由 + 会话创建/文本转发/扇出"
```

---

### Task 8: tsnet 最小接入 (`pkg/engine/tsnet.go`)

**Files:**
- Create: `pkg/engine/tsnet.go`

对应 README §2.4。提供 `Listen()`：当 `TsnetDir` 非空时用 tsnet 起监听，否则退回本地 `net.Listen`（便于本机开发/测试）。不写自动化测试（tsnet 需真实网络与 Auth Key），改为编译验证 + 无头入口手动冒烟。

- [ ] **Step 1: 写实现**

`pkg/engine/tsnet.go`:
```go
package engine

import (
	"fmt"
	"net"

	"tailscale.com/tsnet"
)

// Listen 返回引擎监听用的 net.Listener。
// TsnetDir 为空 → 本地回环监听（开发/测试）；非空 → tsnet 加入 Tailscale 网络。
func (e *Engine) Listen() (net.Listener, func(), error) {
	addr := fmt.Sprintf(":%d", e.cfg.Port)
	if e.cfg.TsnetDir == "" {
		ln, err := net.Listen("tcp", "127.0.0.1"+addr)
		return ln, func() {}, err
	}
	s := &tsnet.Server{
		Hostname: e.cfg.Hostname,
		Dir:      e.cfg.TsnetDir,
	}
	ln, err := s.Listen("tcp", addr)
	if err != nil {
		return nil, func() {}, err
	}
	return ln, func() { _ = s.Close() }, nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./...`
Expected: 无错误。

- [ ] **Step 3: Commit**

```bash
git add pkg/engine/tsnet.go
git commit -m "feat(engine): tsnet 最小接入 (dev 回退本地监听)"
```

---

### Task 9: 无头入口 (`cmd/mac-agent/main.go`)

**Files:**
- Create: `cmd/mac-agent/main.go`

组装引擎：读默认配置 → 校验 claude 版本 → `Listen()` → `http.Serve` WS handler → 捕获 SIGINT/SIGTERM 优雅退出。

- [ ] **Step 1: 写实现**

`cmd/mac-agent/main.go`:
```go
// Command mac-agent 是 Claude Phone 引擎的无头入口（不含 GUI）。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/yang-bin-free/claude-phone/pkg/engine"
)

func main() {
	cfg := engine.DefaultConfig()

	// claude 版本兼容检查
	if ver, err := claudeVersion(cfg.ClaudeBin); err == nil {
		if !cfg.CheckClaudeVersion(ver) {
			log.Fatalf("claude 版本 %s 不兼容 (需要 %s)", ver, cfg.CompatibleRange)
		}
		log.Printf("claude 版本 %s ✓", ver)
	} else {
		log.Printf("警告: 无法执行 claude --version: %v", err)
	}

	e := engine.New(cfg)

	ln, cleanup, err := e.Listen()
	if err != nil {
		log.Fatalf("监听失败: %v", err)
	}
	defer cleanup()
	log.Printf("claude-phone-agent 监听于 %s", ln.Addr())

	mux := http.NewServeMux()
	mux.HandleFunc("/", e.HandleWS)
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()

	// 优雅退出
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("收到退出信号，正在关闭…")
	_ = srv.Shutdown(context.Background())
}

func claudeVersion(bin string) (string, error) {
	out, err := exec.Command(bin, "--version").Output()
	if err != nil {
		return "", err
	}
	// 形如 "2.1.186 (Claude Code)"
	f := strings.Fields(string(out))
	if len(f) > 0 {
		return f[0], nil
	}
	return strings.TrimSpace(string(out)), nil
}
```

- [ ] **Step 2: 编译验证**

Run: `go build ./cmd/mac-agent/`
Expected: 生成二进制，无错误。

- [ ] **Step 3: 冒烟测试（手动，真实 WS 往返）**

写临时客户端 `smoke_test_client.go`（或用现有 engine 测试覆盖），本步骤最小验证进程能起监听：
```bash
go run ./cmd/mac-agent/ &
AGENT_PID=$!
sleep 1
# 端口应在监听
lsof -iTCP:9876 -sTCP:LISTEN || echo "（若用 tsnet 则不在本地 9876）"
kill $AGENT_PID
```
Expected: 日志打印 "监听于 127.0.0.1:9876" 且进程可被 SIGTERM 干净退出。

- [ ] **Step 4: Commit**

```bash
git add cmd/mac-agent/main.go
git commit -m "feat(mac-agent): 无头入口 (版本检查 + 监听 + 优雅退出)"
```

---

### Task 10: 收尾——README 项目结构同步

**Files:**
- Modify: `README.md`（§3 项目结构）

- [ ] **Step 1: 更新项目结构树**

把 `pkg/` 下补上已落地的包，`cmd/` 保留 `mac-agent`。在 README §3 项目结构里，将 `pkg/` 段改为反映真实结构：
```
├── pkg/
│   ├── androidlib/androidlib.go    ← Android Go 核心（gomobile → .aar）
│   ├── protocol/messages.go        ← ★ 三端共享: JSON 消息定义 + 错误码
│   ├── session/                    ← ★ 三端共享: 会话管理 + claude 子进程驱动
│   │   ├── session.go
│   │   ├── claude.go
│   │   └── manager.go
│   └── engine/                     ← Mac 引擎: WS 服务 + tsnet + 配置
│       ├── engine.go
│       ├── wsserver.go
│       ├── config.go
│       └── tsnet.go
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README 项目结构同步引擎核心实际落地"
```

---

## 完成标准 (Definition of Done)

- [ ] `go test ./...` 全绿（protocol / session / engine）
- [ ] `go build ./...` 无错误
- [ ] `cmd/mac-agent` 能起监听、能被信号优雅退出
- [ ] WS 端到端测试覆盖：认证成功/失败、create_session、text → claude 流式扇出、并发上限
- [ ] 无 `ps aux | grep claude` 之类全局清理（README §4.14 已修正为按 PID）
- [ ] 遵守 CLAUDE.md 铁律：无业务层 DB 自增 id 关联（本计划全内存，无 DB，天然不涉及）

## 本计划未覆盖（交给后续计划/阶段）

- Plan 2：Mac 桌面 GUI（systray + webview + 聊天区 + 管理区 + adminproto）
- 权限请求全链路（permission_request / batch / rule 持久化，README §4.9）
- 消息历史持久化 messages.jsonl + load_history（README §4.12）
- Agent 崩溃恢复扫描 dormant（README §4.14）
- 首次配对生成 device token / Auth Key（README §4.13）
- caffeinate 防睡眠、健康监控、配置热加载 fsnotify
- 手机端 Tailscale 引擎与真实跨网络端到端
