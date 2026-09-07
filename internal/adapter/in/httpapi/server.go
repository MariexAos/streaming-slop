package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"streaming-agent/internal/flow"
	"streaming-agent/internal/monitoring"
	"streaming-agent/internal/session"
)

type Controller interface {
	Start(context.Context) error
	Stop(context.Context) error
	ForceFallback(context.Context, bool) error
}

type FlowController interface {
	CurrentFlow() flow.FlowRun
}

type PreviewController interface {
	PreviewAsset(string) (string, bool)
}

type HistoryController interface {
	ListSessionHistory(context.Context) ([]session.HistorySummary, error)
	SessionHistory(context.Context, string) (session.History, error)
}

type GenerationConfig struct {
	Provider              string  `json:"provider"`
	BaseURL               string  `json:"baseUrl"`
	Model                 string  `json:"model"`
	Resolution            string  `json:"resolution"`
	DurationSeconds       int     `json:"durationSeconds"`
	Ratio                 string  `json:"ratio"`
	APIKeyConfigured      bool    `json:"apiKeyConfigured"`
	UnitPriceCNYPerSecond float64 `json:"unitPriceCnyPerSecond"`
}

type GenerationConfigUpdate struct {
	Model           string `json:"model"`
	Resolution      string `json:"resolution"`
	DurationSeconds int    `json:"durationSeconds"`
	Ratio           string `json:"ratio"`
	APIKey          string `json:"apiKey,omitempty"`
}

type ConfigController interface {
	GenerationConfig() (GenerationConfig, error)
	UpdateGenerationConfig(context.Context, GenerationConfigUpdate) (GenerationConfig, error)
}

type BilibiliConfig struct {
	RoomID           int     `json:"roomId"`
	CookieConfigured bool    `json:"cookieConfigured"`
	Status           string  `json:"status"`
	LastError        *string `json:"lastError"`
}

type BilibiliConfigUpdate struct {
	RoomID int    `json:"roomId"`
	Cookie string `json:"cookie,omitempty"`
}

type MockDanmaku struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

type BilibiliConfigController interface {
	BilibiliConfig() BilibiliConfig
	UpdateBilibiliConfig(context.Context, BilibiliConfigUpdate) (BilibiliConfig, error)
	InjectMockDanmaku(context.Context, MockDanmaku) error
}

type QwenConfig struct {
	BaseURL          string `json:"baseUrl"`
	ObserverModel    string `json:"observerModel"`
	DirectorModel    string `json:"directorModel"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
}

type QwenConfigUpdate struct {
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey,omitempty"`
}

type QwenConfigController interface {
	QwenConfig() QwenConfig
	UpdateQwenConfig(context.Context, QwenConfigUpdate) (QwenConfig, error)
}

type Server struct {
	handler http.Handler
}

func New(controller Controller, hub *monitoring.Hub, metrics http.Handler, ui http.Handler, configController ConfigController, bilibiliController BilibiliConfigController, qwenController QwenConfigController) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.Handle("GET /metrics", metrics)
	mux.HandleFunc("GET /api/v1/ops/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, hub.Current())
	})
	mux.HandleFunc("GET /api/v1/ops/events", events(hub))
	mux.HandleFunc("GET /api/v1/ops/media/{segmentID}", func(w http.ResponseWriter, r *http.Request) {
		previewController, ok := controller.(PreviewController)
		if !ok {
			http.NotFound(w, r)
			return
		}
		path, ok := previewController.PreviewAsset(r.PathValue("segmentID"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, path)
	})
	mux.HandleFunc("GET /api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		historyController, ok := controller.(HistoryController)
		if !ok {
			http.NotFound(w, r)
			return
		}
		items, err := historyController.ListSessionHistory(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "history_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Sessions []session.HistorySummary `json:"sessions"`
		}{Sessions: items})
	})
	mux.HandleFunc("GET /api/v1/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		historyController, ok := controller.(HistoryController)
		if !ok {
			http.NotFound(w, r)
			return
		}
		item, err := historyController.SessionHistory(r.Context(), r.PathValue("id"))
		if errors.Is(err, session.ErrHistoryNotFound) {
			writeError(w, http.StatusNotFound, "session_not_found", "session was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "history_unavailable", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	})
	mux.HandleFunc("GET /api/v1/flows", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, struct {
			Flows []flow.FlowDefinition `json:"flows"`
		}{Flows: []flow.FlowDefinition{flow.HostDialogueFlow()}})
	})
	mux.HandleFunc("GET /api/v1/flows/current", func(w http.ResponseWriter, _ *http.Request) {
		current := flow.IdleHostDialogueRun()
		if flowController, ok := controller.(FlowController); ok {
			current = flowController.CurrentFlow()
		}
		writeJSON(w, http.StatusOK, current)
	})
	mux.HandleFunc("POST /api/v1/ops/start", command("start", func(ctx context.Context) error {
		return controller.Start(ctx)
	}))
	mux.HandleFunc("POST /api/v1/ops/stop", command("stop", func(ctx context.Context) error {
		return controller.Stop(ctx)
	}))
	mux.HandleFunc("PUT /api/v1/ops/fallback", fallbackCommand(controller))
	if configController != nil {
		mux.HandleFunc("GET /api/v1/config/generation", getGenerationConfig(configController))
		mux.HandleFunc("PUT /api/v1/config/generation", updateGenerationConfig(configController))
	}
	if bilibiliController != nil {
		mux.HandleFunc("GET /api/v1/config/bilibili", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, bilibiliController.BilibiliConfig())
		})
		mux.HandleFunc("PUT /api/v1/config/bilibili", updateBilibiliConfig(bilibiliController))
		mux.HandleFunc("POST /api/v1/config/bilibili/mock", injectMockDanmaku(bilibiliController))
	}
	if qwenController != nil {
		mux.HandleFunc("GET /api/v1/config/qwen", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, qwenController.QwenConfig())
		})
		mux.HandleFunc("PUT /api/v1/config/qwen", updateQwenConfig(qwenController))
	}
	if ui != nil {
		mux.Handle("/", ui)
	}
	return &Server{handler: mux}
}

func updateQwenConfig(controller QwenConfigController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		var update QwenConfigUpdate
		if err := decoder.Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid Qwen config")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		config, err := controller.UpdateQwenConfig(r.Context(), update)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, config)
	}
}

func injectMockDanmaku(controller BilibiliConfigController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		decoder := json.NewDecoder(io.LimitReader(r.Body, 2048))
		decoder.DisallowUnknownFields()
		var message MockDanmaku
		if err := decoder.Decode(&message); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid mock danmaku")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		if err := controller.InjectMockDanmaku(r.Context(), message); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_mock_danmaku", err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"command": "mock-danmaku", "status": "accepted", "acceptedAt": time.Now().UTC(),
		})
	}
}

func updateBilibiliConfig(controller BilibiliConfigController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		var update BilibiliConfigUpdate
		if err := decoder.Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid Bilibili config")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		config, err := controller.UpdateBilibiliConfig(r.Context(), update)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, config)
	}
}

func getGenerationConfig(controller ConfigController) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		config, err := controller.GenerationConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "config_error", "generation config is unavailable")
			return
		}
		writeJSON(w, http.StatusOK, config)
	}
}

func updateGenerationConfig(controller ConfigController) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
		decoder.DisallowUnknownFields()
		var update GenerationConfigUpdate
		if err := decoder.Decode(&update); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid generation config")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		config, err := controller.UpdateGenerationConfig(r.Context(), update)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_config", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, config)
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func command(name string, run func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := requireEmptyObject(r.Body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if err := run(r.Context()); err != nil {
			writeCommandError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"command": name, "status": "accepted", "acceptedAt": time.Now().UTC(),
		})
	}
}

func fallbackCommand(controller Controller) http.HandlerFunc {
	type request struct {
		Forced *bool `json:"forced"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() { _ = r.Body.Close() }()
		decoder := json.NewDecoder(io.LimitReader(r.Body, 1024))
		decoder.DisallowUnknownFields()
		var body request
		if err := decoder.Decode(&body); err != nil || body.Forced == nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain forced boolean")
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "body must contain one JSON object")
			return
		}
		if err := controller.ForceFallback(r.Context(), *body.Forced); err != nil {
			writeCommandError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"command": "fallback", "status": "accepted", "acceptedAt": time.Now().UTC(),
		})
	}
}

func events(hub *monitoring.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			writeError(w, http.StatusInternalServerError, "stream_unsupported", "response streaming is unavailable")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")

		updates, unsubscribe := hub.Subscribe()
		defer unsubscribe()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case snapshot, open := <-updates:
				if !open {
					return
				}
				payload, err := json.Marshal(snapshot)
				if err != nil {
					return
				}
				if _, err := fmt.Fprintf(w, "id: %d\nevent: snapshot\ndata: %s\n\n", snapshot.Revision, payload); err != nil {
					return
				}
				flusher.Flush()
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

func requireEmptyObject(body io.ReadCloser) error {
	defer func() { _ = body.Close() }()
	decoder := json.NewDecoder(io.LimitReader(body, 1024))
	decoder.DisallowUnknownFields()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return errors.New("body must be an empty JSON object")
	}
	if len(value) != 0 {
		return errors.New("body must be an empty JSON object")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain one JSON object")
	}
	return nil
}

func writeCommandError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, session.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", err.Error())
	case errors.Is(err, session.ErrUnavailable):
		writeError(w, http.StatusServiceUnavailable, "unavailable", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "runtime command failed")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSONStatus(w, status, map[string]string{"code": code, "message": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONStatus(w, status, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func IsLoopbackAddress(address string) bool {
	host, _, found := strings.Cut(address, ":")
	return found && (host == "127.0.0.1" || host == "localhost" || host == "[::1]")
}
