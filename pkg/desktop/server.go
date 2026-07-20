package desktop

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"mime"
	"net"
	"net/http"

	"github.com/yang-bin-free/claude-phone/pkg/protocol"
	webassets "github.com/yang-bin-free/claude-phone/web"
)

type AppStatus struct {
	Ready         bool   `json:"ready"`
	Paused        bool   `json:"paused"`
	ClaudeBin     string `json:"claudeBin,omitempty"`
	ClaudeVersion string `json:"claudeVersion,omitempty"`
	Error         string `json:"error,omitempty"`
}

type HandlerOptions struct {
	EngineHandler func() http.Handler
	AdminHandler  func() http.Handler
	Status        func() AppStatus
	AddProject    func(string) (protocol.ProjectInfo, error)
	AdminToken    string
}

func NewHandler(options HandlerOptions) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/ws", availableHandler(options.EngineHandler))
	mux.Handle("/admin/", availableHandler(options.AdminHandler))
	mux.HandleFunc("GET /desktop/status", func(w http.ResponseWriter, _ *http.Request) {
		status := AppStatus{}
		if options.Status != nil {
			status = options.Status()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	})
	mux.HandleFunc("POST /desktop/projects", func(w http.ResponseWriter, r *http.Request) {
		mediaType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		providedToken := r.Header.Get("X-CodeAfar-Admin-Token")
		if !isLoopbackRequest(r) || mediaType != "application/json" || options.AdminToken == "" ||
			subtle.ConstantTimeCompare([]byte(providedToken), []byte(options.AdminToken)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if options.AddProject == nil {
			http.Error(w, "desktop engine unavailable", http.StatusServiceUnavailable)
			return
		}
		var request struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&request); err != nil || request.Path == "" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		project, err := options.AddProject(request.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(project)
	})
	mux.HandleFunc("/", serveAsset("chat/index.html", "text/html"))
	mux.HandleFunc("/assets/chat.js", serveAsset("chat/chat.js", "text/javascript"))
	mux.HandleFunc("/assets/tool-format.js", serveAsset("chat/tool-format.js", "text/javascript"))
	mux.HandleFunc("/assets/admin.js", serveAsset("admin/admin.js", "text/javascript"))
	mux.HandleFunc("/assets/core.css", serveAsset("chat/core.css", "text/css"))
	mux.HandleFunc("/assets/desktop.css", serveAsset("chat/desktop.css", "text/css"))
	mux.HandleFunc("/assets/mobile.css", serveAsset("chat/mobile.css", "text/css"))
	mux.HandleFunc("/assets/admin.css", serveAsset("admin/admin.css", "text/css"))
	return mux
}

func isLoopbackRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func availableHandler(provider func() http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if provider == nil {
			http.Error(w, "desktop engine unavailable", http.StatusServiceUnavailable)
			return
		}
		handler := provider()
		if handler == nil {
			http.Error(w, "desktop engine unavailable", http.StatusServiceUnavailable)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

func serveAsset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		content, err := fs.ReadFile(webassets.Assets, name)
		if err != nil {
			http.Error(w, "asset not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(content)
	}
}
