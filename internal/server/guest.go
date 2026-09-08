package server

import (
	"net/http"
	"strings"

	"streaming-agent/internal/monitoring"
)

// GuestHandler exposes only viewing and audience input on the LAN listener.
func GuestHandler(hub *monitoring.Hub, controller BilibiliConfigController, preview, assets http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/guest/status", func(w http.ResponseWriter, r *http.Request) {
		snapshot := hub.Current()
		message := snapshot.Interaction
		if strings.HasPrefix(message, "互动未采用：") {
			message = "互动暂未采用，请稍后再试"
		}
		writeJSON(w, http.StatusOK, map[string]any{"streamStatus": snapshot.Stream.Status, "interactive": snapshot.Controls.CanStop, "message": message})
	})
	send := injectMockDanmaku(controller)
	mux.HandleFunc("POST /api/guest/messages", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			writeError(w, http.StatusUnsupportedMediaType, "invalid_request", "请使用 JSON 提交互动")
			return
		}
		send(w, r)
	})
	mux.Handle("GET /live-preview/", preview)
	mux.Handle("GET /assets/", assets)
	mux.Handle("GET /guest", assets)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/guest", http.StatusTemporaryRedirect)
	})
	return mux
}
