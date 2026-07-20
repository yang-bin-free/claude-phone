package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	"gopkg.in/yaml.v3"
)

type projectStore struct {
	mu   sync.Mutex
	path string
}

// AddProject authorizes a local directory selected by the desktop shell.
func (e *Engine) AddProject(path string) (protocol.ProjectInfo, error) {
	permission := e.runtimeConfig().DefaultPermission
	project, err := e.projects.Add(adminproto.Project{Path: path, Permission: permission})
	if err != nil {
		return protocol.ProjectInfo{}, err
	}
	return protocol.ProjectInfo{Name: project.Name, Path: project.Path, Permission: project.Permission}, nil
}

type projectsFile struct {
	Projects []adminproto.Project `yaml:"projects"`
}

func newProjectStore(dataDir string) *projectStore {
	return &projectStore{path: filepath.Join(dataDir, "projects.yaml")}
}

func (s *projectStore) List() ([]adminproto.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *projectStore) Add(project adminproto.Project) (adminproto.Project, error) {
	info, err := os.Stat(project.Path)
	if err != nil || !info.IsDir() || !filepath.IsAbs(project.Path) {
		return adminproto.Project{}, errors.New("project path must be an existing absolute directory")
	}
	if project.Name == "" {
		project.Name = filepath.Base(project.Path)
	}
	project.Path = filepath.Clean(project.Path)
	project.ProjectID = projectPathID(project.Path)

	s.mu.Lock()
	defer s.mu.Unlock()
	projects, err := s.loadLocked()
	if err != nil {
		return adminproto.Project{}, err
	}
	replaced := false
	for i := range projects {
		if projects[i].Path == project.Path {
			projects[i] = project
			replaced = true
		}
	}
	if !replaced {
		projects = append(projects, project)
	}
	return project, s.saveLocked(projects)
}

func (s *projectStore) Delete(projectID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	projects, err := s.loadLocked()
	if err != nil {
		return false, err
	}
	filtered := projects[:0]
	found := false
	for _, project := range projects {
		if project.ProjectID == projectID {
			found = true
			continue
		}
		filtered = append(filtered, project)
	}
	if !found {
		return false, nil
	}
	return true, s.saveLocked(filtered)
}

func (s *projectStore) loadLocked() ([]adminproto.Project, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []adminproto.Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file projectsFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	for i := range file.Projects {
		file.Projects[i].Path = filepath.Clean(file.Projects[i].Path)
		file.Projects[i].ProjectID = projectPathID(file.Projects[i].Path)
	}
	sort.Slice(file.Projects, func(i, j int) bool { return file.Projects[i].Name < file.Projects[j].Name })
	return file.Projects, nil
}

func (s *projectStore) saveLocked(projects []adminproto.Project) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), "projects-*.yaml")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	err = yaml.NewEncoder(temp).Encode(projectsFile{Projects: projects})
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tempName, s.path)
}

func projectPathID(path string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(path)))
	return hex.EncodeToString(sum[:8])
}
