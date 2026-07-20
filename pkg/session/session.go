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
	Permission string
	Provider   string
	Model      string
	Status     string // active | dormant | stopped
	CreatedAt  int64

	mu                sync.RWMutex
	providerSessionID string
	subs              map[string]struct{}
	sender            SenderFunc
}

// NewSession 创建会话，owner 自动成为首个订阅者。
func NewSession(id, name, cwd, owner string) *Session {
	return &Session{
		ID:       id,
		Name:     name,
		Cwd:      cwd,
		Owner:    owner,
		Provider: "claude",
		Status:   "active",
		subs:     map[string]struct{}{owner: {}},
	}
}

// SetSender 注入发送回调。
func (s *Session) SetSender(fn SenderFunc) {
	s.mu.Lock()
	s.sender = fn
	s.mu.Unlock()
}

// Subscribe 把设备加入订阅者集合。
func (s *Session) Subscribe(deviceID string) {
	s.mu.Lock()
	s.subs[deviceID] = struct{}{}
	if s.Status != "stopped" {
		s.Status = "active"
	}
	s.mu.Unlock()
}

// Unsubscribe 把设备移出订阅者集合。
func (s *Session) Unsubscribe(deviceID string) {
	s.mu.Lock()
	delete(s.subs, deviceID)
	if len(s.subs) == 0 && s.Status != "stopped" {
		s.Status = "dormant"
	}
	s.mu.Unlock()
}

// SetStatus 更新会话状态。
func (s *Session) SetStatus(status string) {
	s.mu.Lock()
	s.Status = status
	s.mu.Unlock()
}

func (s *Session) SetPermission(permission string) {
	s.mu.Lock()
	s.Permission = permission
	s.mu.Unlock()
}

func (s *Session) PermissionMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Permission
}

// SetProviderSessionID records an upstream provider conversation ID and
// reports whether it changed. Empty IDs are never accepted.
func (s *Session) SetProviderSessionID(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || s.providerSessionID == id {
		return false
	}
	s.providerSessionID = id
	return true
}

func (s *Session) ProviderSessionIdentity() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerSessionID
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
