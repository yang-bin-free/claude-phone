package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	// ErrSessionLimit 表示并发会话数达到上限（对应 SESSION_LIMIT_REACHED）。
	ErrSessionLimit = errors.New("session limit reached")

	// ErrSessionNotFound 表示会话不存在（对应 SESSION_NOT_FOUND）。
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionNotOwner 表示当前设备不是会话 owner（对应 SESSION_NOT_OWNER）。
	ErrSessionNotOwner = errors.New("session not owner")
)

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

// NewManager 按配置创建会话管理器。
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 5
	}
	if cfg.Now == nil {
		cfg.Now = func() int64 { return time.Now().Unix() }
	}
	if cfg.IDFunc == nil {
		cfg.IDFunc = func() string {
			var b [16]byte
			if _, err := rand.Read(b[:]); err != nil {
				return time.Now().Format("20060102150405.000000000")
			}
			return "sess-" + hex.EncodeToString(b[:])
		}
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

// Get 按 ID 查会话。
func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.byID[id]
	return s, ok
}

// Remove 删除会话。
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt == out[j].CreatedAt {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt < out[j].CreatedAt
	})
	return out
}
