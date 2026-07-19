package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"gopkg.in/yaml.v3"
)

type templateStore struct {
	mu   sync.Mutex
	path string
}

type templatesFile struct {
	Templates []protocol.TemplateInfo `yaml:"templates"`
}

func newTemplateStore(dataDir string) *templateStore {
	return &templateStore{path: filepath.Join(dataDir, "templates.yaml")}
}

func (s *templateStore) List() ([]protocol.TemplateInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *templateStore) Add(item protocol.TemplateInfo) (protocol.TemplateInfo, error) {
	item.Label = strings.TrimSpace(item.Label)
	item.Prompt = strings.TrimSpace(item.Prompt)
	if item.Label == "" || item.Prompt == "" {
		return protocol.TemplateInfo{}, errors.New("template label and prompt are required")
	}
	item.TemplateID = templateID(item.Label, item.Prompt)
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.loadLocked()
	if err != nil {
		return protocol.TemplateInfo{}, err
	}
	for i := range items {
		if items[i].TemplateID == item.TemplateID || items[i].Label == item.Label {
			items[i] = item
			return item, s.saveLocked(items)
		}
	}
	items = append(items, item)
	return item, s.saveLocked(items)
}

func (s *templateStore) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.loadLocked()
	if err != nil {
		return false, err
	}
	filtered := items[:0]
	found := false
	for _, item := range items {
		if item.TemplateID == id {
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		return false, nil
	}
	return true, s.saveLocked(filtered)
}

func (s *templateStore) loadLocked() ([]protocol.TemplateInfo, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []protocol.TemplateInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file templatesFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	result := make([]protocol.TemplateInfo, 0, len(file.Templates))
	for _, item := range file.Templates {
		if item.Label != "" && item.Prompt != "" {
			item.TemplateID = templateID(item.Label, item.Prompt)
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *templateStore) saveLocked(items []protocol.TemplateInfo) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), "templates-*.yaml")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	err = yaml.NewEncoder(temp).Encode(templatesFile{Templates: items})
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tempName, s.path)
}

func templateID(label, prompt string) string {
	sum := sha256.Sum256([]byte(label + "\x00" + prompt))
	return hex.EncodeToString(sum[:8])
}
