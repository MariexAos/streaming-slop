package webui

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var embedded embed.FS

func ReadAsset(name string) ([]byte, error) {
	return embedded.ReadFile(path.Join("dist", name))
}

// NewHandler serves the embedded Vite build and falls back to index.html for SPA routes.
func NewHandler() http.Handler {
	assets, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}

	return &handler{
		assets: assets,
		files:  http.FileServer(http.FS(assets)),
	}
}

type handler struct {
	assets fs.FS
	files  http.Handler
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if reservedPath(requestPath) || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimPrefix(requestPath, "/")
	if name != "" && name != "." {
		info, err := fs.Stat(h.assets, name)
		if err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			h.files.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(name, "assets/") {
			http.NotFound(w, r)
			return
		}
	}

	index, err := fs.ReadFile(h.assets, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
}

func reservedPath(requestPath string) bool {
	for _, prefix := range []string{"/api", "/metrics", "/healthz"} {
		if requestPath == prefix || strings.HasPrefix(requestPath, prefix+"/") {
			return true
		}
	}
	return false
}
