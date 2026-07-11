package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
)

type permissionStore struct {
	mu   sync.Mutex
	path string
}

func newPermissionStore(dataDir string) *permissionStore {
	return &permissionStore{path: filepath.Join(dataDir, "permission-rules.json")}
}

func (s *permissionStore) List() []adminproto.PermissionRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, _ := s.loadLocked()
	return rules
}

func (s *permissionStore) Add(tool, pattern string) (adminproto.PermissionRule, error) {
	tool, pattern = strings.TrimSpace(tool), strings.TrimSpace(pattern)
	if tool == "" || strings.ContainsAny(tool, "()\r\n") || strings.ContainsAny(pattern, "\r\n") {
		return adminproto.PermissionRule{}, errors.New("invalid permission rule")
	}
	rule := adminproto.PermissionRule{RuleID: permissionRuleID(tool, pattern), Tool: tool, Pattern: pattern}
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadLocked()
	if err != nil {
		return adminproto.PermissionRule{}, err
	}
	for i := range rules {
		if rules[i].RuleID == rule.RuleID {
			rules[i] = rule
			return rule, s.saveLocked(rules)
		}
	}
	rules = append(rules, rule)
	return rule, s.saveLocked(rules)
}

func (s *permissionStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rules, err := s.loadLocked()
	if err != nil {
		return false
	}
	next := rules[:0]
	found := false
	for _, rule := range rules {
		if rule.RuleID == id {
			found = true
		} else {
			next = append(next, rule)
		}
	}
	if !found || s.saveLocked(next) != nil {
		return false
	}
	return true
}

func (s *permissionStore) AllowedTools() []string {
	rules := s.List()
	tools := make([]string, 0, len(rules))
	for _, rule := range rules {
		value := rule.Tool
		if rule.Pattern != "" {
			value += "(" + rule.Pattern + ")"
		}
		tools = append(tools, value)
	}
	return tools
}

func (s *permissionStore) loadLocked() ([]adminproto.PermissionRule, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []adminproto.PermissionRule{}, nil
	}
	if err != nil {
		return nil, err
	}
	var rules []adminproto.PermissionRule
	if err := json.Unmarshal(b, &rules); err != nil {
		return nil, err
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].RuleID < rules[j].RuleID })
	return rules, nil
}

func (s *permissionStore) saveLocked(rules []adminproto.PermissionRule) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(s.path, b, 0o600)
}

func permissionRuleID(tool, pattern string) string {
	sum := sha256.Sum256([]byte(tool + "\x00" + pattern))
	return hex.EncodeToString(sum[:8])
}
