package desktop

import (
	"io/fs"
	"net/http"

	webassets "github.com/yang-bin-free/claude-phone/web"
)

func NewHandler(engineHandler, adminHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/ws", engineHandler)
	mux.Handle("/admin/", adminHandler)
	mux.HandleFunc("/", serveAsset("chat/index.html", "text/html"))
	mux.HandleFunc("/assets/chat.js", serveAsset("chat/chat.js", "text/javascript"))
	mux.HandleFunc("/assets/admin.js", serveAsset("admin/admin.js", "text/javascript"))
	mux.HandleFunc("/assets/core.css", serveAsset("chat/core.css", "text/css"))
	mux.HandleFunc("/assets/desktop.css", serveAsset("chat/desktop.css", "text/css"))
	mux.HandleFunc("/assets/admin.css", serveAsset("admin/admin.css", "text/css"))
	return mux
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
