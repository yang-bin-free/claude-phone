package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
	"gopkg.in/yaml.v3"
)

type deviceStore struct {
	mu   sync.Mutex
	path string
}

type deviceRecord struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
}

type devicesFile struct {
	Devices []deviceRecord `yaml:"devices"`
}

func newDeviceStore(dataDir string) *deviceStore {
	return &deviceStore{path: filepath.Join(dataDir, "devices.yaml")}
}

func GenerateDeviceCredential(dataDir, name string) (adminproto.DeviceCredential, error) {
	cfg := Config{DataDir: dataDir}.withDefaults()
	return newDeviceStore(cfg.DataDir).Add(name)
}

func (s *deviceStore) Add(name string) (adminproto.DeviceCredential, error) {
	if name == "" {
		name = "Device"
	}
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return adminproto.DeviceCredential{}, err
	}
	token := "dt_" + hex.EncodeToString(value[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadLocked()
	if err != nil {
		return adminproto.DeviceCredential{}, err
	}
	records = append(records, deviceRecord{Name: name, Token: token})
	if err := s.saveLocked(records); err != nil {
		return adminproto.DeviceCredential{}, err
	}
	return adminproto.DeviceCredential{Device: adminproto.DeviceSnapshot{DeviceID: deviceTokenID(token), Name: name}, DeviceToken: token}, nil
}

func (s *deviceStore) Lookup(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadLocked()
	if err != nil {
		return "", false
	}
	for _, record := range records {
		if record.Token == token {
			return record.Name, true
		}
	}
	return "", false
}

func (s *deviceStore) List() []deviceRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, _ := s.loadLocked()
	return records
}

func (s *deviceStore) Delete(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadLocked()
	if err != nil {
		return false
	}
	filtered := records[:0]
	found := false
	for _, record := range records {
		if deviceTokenID(record.Token) == deviceID {
			found = true
			continue
		}
		filtered = append(filtered, record)
	}
	return found && s.saveLocked(filtered) == nil
}

func (s *deviceStore) loadLocked() ([]deviceRecord, error) {
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []deviceRecord{}, nil
	}
	if err != nil {
		return nil, err
	}
	var file devicesFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		return nil, err
	}
	return file.Devices, nil
}

func (s *deviceStore) saveLocked(records []deviceRecord) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), "devices-*.yaml")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	err = yaml.NewEncoder(temp).Encode(devicesFile{Devices: records})
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, s.path)
}
