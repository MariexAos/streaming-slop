package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"streaming-agent/internal/monitoring"
)

func TestGuestCannotAccessManagementRoutes(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 0, 0, 0))
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := GuestHandler(hub, &bilibiliControllerStub{}, downstream, downstream)
	for _, path := range []string{"/api/v1/config/services", "/api/v1/ops/start", "/api/v1/budget", "/metrics", "/healthz"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("management path %s exposed: %d", path, response.Code)
		}
	}
	for _, path := range []string{"/guest", "/live-preview/index.m3u8", "/api/guest/status"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("guest path %s unavailable: %d", path, response.Code)
		}
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/guest/messages", strings.NewReader(`{"username":"朋友","text":"你好"}`))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("message rejected: %d", response.Code)
	}
}
