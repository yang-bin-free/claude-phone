package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/session"
)

type historyStore struct {
	mu   sync.Mutex
	root string
}

type sessionMeta struct {
	SessionID  string `json:"sessionId"`
	Name       string `json:"name"`
	Cwd        string `json:"cwd"`
	Owner      string `json:"owner"`
	Permission string `json:"permission"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	CreatedAt  int64  `json:"createdAt"`
}

func newHistoryStore(dataDir string) *historyStore {
	return &historyStore{root: filepath.Join(dataDir, "sessions")}
}

func (s *historyStore) CreateSession(sess *session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, sess.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	meta := sessionMeta{SessionID: sess.ID, Name: sess.Name, Cwd: sess.Cwd, Owner: sess.Owner, Permission: sess.Permission, Provider: sess.Provider, Model: sess.Model, CreatedAt: sess.CreatedAt}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(dir, "meta.json"), b, 0o600)
}

func (s *historyStore) Append(sessionID string, payload []byte) error {
	if !json.Valid(payload) {
		return errors.New("history payload must be valid JSON")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.root, sessionID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "messages.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(bytes.Clone(payload), '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (s *historyStore) Load(sessionID string, limit int) ([]json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(filepath.Join(s.root, sessionID, "messages.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return []json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var messages []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := bytes.Clone(scanner.Bytes())
		if json.Valid(line) {
			messages = append(messages, json.RawMessage(line))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return messages, nil
}

func (s *historyStore) Restore() ([]*session.Session, error) {
	entries, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var sessions []*session.Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.root, entry.Name(), "meta.json"))
		if err != nil {
			continue
		}
		var meta sessionMeta
		if json.Unmarshal(b, &meta) != nil || meta.SessionID == "" {
			continue
		}
		sess := session.NewSession(meta.SessionID, meta.Name, meta.Cwd, meta.Owner)
		sess.Permission = meta.Permission
		if meta.Provider != "" {
			sess.Provider = meta.Provider
		}
		sess.Model = meta.Model
		sess.CreatedAt = meta.CreatedAt
		sess.SetStatus("dormant")
		sessions = append(sessions, sess)
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].CreatedAt < sessions[j].CreatedAt })
	return sessions, nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
