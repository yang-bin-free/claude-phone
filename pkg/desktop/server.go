package desktop

import (
	"encoding/json"
	"io/fs"
	"net/http"

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
	mux.HandleFunc("/", serveAsset("chat/index.html", "text/html"))
	mux.HandleFunc("/assets/chat.js", serveAsset("chat/chat.js", "text/javascript"))
	mux.HandleFunc("/assets/admin.js", serveAsset("admin/admin.js", "text/javascript"))
	mux.HandleFunc("/assets/core.css", serveAsset("chat/core.css", "text/css"))
	mux.HandleFunc("/assets/desktop.css", serveAsset("chat/desktop.css", "text/css"))
	mux.HandleFunc("/assets/mobile.css", serveAsset("chat/mobile.css", "text/css"))
	mux.HandleFunc("/assets/admin.css", serveAsset("admin/admin.css", "text/css"))
	return mux
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
