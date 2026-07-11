package engine

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sort"

	"github.com/yang-bin-free/claude-phone/pkg/adminproto"
)

const maxAdminBodyBytes = 64 << 10

func (e *Engine) AdminHandler(token string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/status", func(w http.ResponseWriter, _ *http.Request) {
		projects, err := e.projects.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeAdminJSON(w, http.StatusOK, adminproto.Snapshot{Agent: adminStatus(e.Status()), Devices: e.adminDevices(), Projects: projects})
	})
	mux.HandleFunc("POST /admin/projects", func(w http.ResponseWriter, r *http.Request) {
		var project adminproto.Project
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBodyBytes)).Decode(&project); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		created, err := e.projects.Add(project)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeAdminJSON(w, http.StatusCreated, created)
	})
	mux.HandleFunc("DELETE /admin/projects/{projectID}", func(w http.ResponseWriter, r *http.Request) {
		found, err := e.projects.Delete(r.PathValue("projectID"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "project not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /admin/devices/{deviceID}", func(w http.ResponseWriter, r *http.Request) {
		if !e.revokeDevice(r.PathValue("deviceID")) {
			http.Error(w, "device not found", http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /admin/sessions/stop", func(w http.ResponseWriter, r *http.Request) {
		var request adminproto.StopSessionRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAdminBodyBytes))
		if err := decoder.Decode(&request); err != nil || request.SessionID == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		s, ok := e.manager.Get(request.SessionID)
		if !ok {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		if err := e.stopSession(s); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	want := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemote(r.RemoteAddr) || !constantTimeHeaderMatch(r.Header.Get("Authorization"), want) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (e *Engine) adminDevices() []adminproto.DeviceSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()
	devices := make([]adminproto.DeviceSnapshot, 0, len(e.cfg.DeviceTokens))
	for token, name := range e.cfg.DeviceTokens {
		_, online := e.clients[token]
		devices = append(devices, adminproto.DeviceSnapshot{DeviceID: deviceTokenID(token), Name: name, Online: online})
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices
}

func (e *Engine) revokeDevice(deviceID string) bool {
	var revokedToken string
	var connection *client
	e.mu.Lock()
	for token := range e.cfg.DeviceTokens {
		if deviceTokenID(token) == deviceID {
			revokedToken = token
			delete(e.cfg.DeviceTokens, token)
			connection = e.clients[token]
			delete(e.clients, token)
			break
		}
	}
	e.mu.Unlock()
	if revokedToken == "" {
		return false
	}
	for _, s := range e.manager.List() {
		s.Unsubscribe(revokedToken)
	}
	if connection != nil && connection.conn != nil {
		_ = connection.conn.Close()
	}
	return true
}

func deviceTokenID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

func isLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func constantTimeHeaderMatch(header string, want [sha256.Size]byte) bool {
	got := sha256.Sum256([]byte(header))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

func adminStatus(status StatusReport) adminproto.AgentStatus {
	sessions := make([]adminproto.SessionSnapshot, 0, len(status.Sessions))
	for _, s := range status.Sessions {
		sessions = append(sessions, adminproto.SessionSnapshot{
			SessionID: s.SessionID, Name: s.Name, Status: s.Status, Owner: s.Owner,
			Subscribers: s.Subscribers, CreatedAt: s.CreatedAt, Running: s.Running,
		})
	}
	return adminproto.AgentStatus{
		Addr: status.Addr, AgentVersion: status.AgentVersion, ClaudeVersion: status.ClaudeVersion,
		ClaudeBin: status.ClaudeBin, DefaultWorkingDir: status.DefaultWorkingDir,
		DefaultPermission: status.DefaultPermission, MaxConcurrentSession: status.MaxConcurrentSession,
		ConnectedDevices: status.ConnectedDevices, Sessions: sessions,
	}
}

func writeAdminJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
