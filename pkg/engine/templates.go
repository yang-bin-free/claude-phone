package engine

import (
	"errors"
	"os"
	"path/filepath"
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
			result = append(result, item)
		}
	}
	return result, nil
}
