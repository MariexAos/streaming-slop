package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"streaming-agent/internal/flow"
	"streaming-agent/internal/monitoring"
)

type controllerStub struct {
	started  int
	stopped  int
	fallback *bool
	preview  string
}

type configControllerStub struct{ config GenerationConfig }
type bilibiliControllerStub struct{ config BilibiliConfig }
type qwenControllerStub struct{ config QwenConfig }

func (c *configControllerStub) GenerationConfig() (GenerationConfig, error) { return c.config, nil }
func (c *configControllerStub) UpdateGenerationConfig(_ context.Context, update GenerationConfigUpdate) (GenerationConfig, error) {
	c.config.Model = update.Model
	c.config.Resolution = update.Resolution
	c.config.DurationSeconds = update.DurationSeconds
	c.config.Ratio = update.Ratio
	return c.config, nil
}
func (c *bilibiliControllerStub) BilibiliConfig() BilibiliConfig { return c.config }
func (c *bilibiliControllerStub) UpdateBilibiliConfig(_ context.Context, update BilibiliConfigUpdate) (BilibiliConfig, error) {
	c.config.RoomID = update.RoomID
	c.config.CookieConfigured = c.config.CookieConfigured || update.Cookie != ""
	c.config.Status = "connecting"
	return c.config, nil
}
func (c *bilibiliControllerStub) InjectMockDanmaku(_ context.Context, message MockDanmaku) error {
	if strings.TrimSpace(message.Text) == "" {
		return errors.New("danmaku text is required")
	}
	return nil
}
func (c *qwenControllerStub) QwenConfig() QwenConfig { return c.config }
func (c *qwenControllerStub) UpdateQwenConfig(_ context.Context, update QwenConfigUpdate) (QwenConfig, error) {
	c.config.BaseURL = update.BaseURL
	c.config.APIKeyConfigured = c.config.APIKeyConfigured || update.APIKey != ""
	return c.config, nil
}

func (c *controllerStub) Start(context.Context) error { c.started++; return nil }
func (c *controllerStub) Stop(context.Context) error  { c.stopped++; return nil }
func (c *controllerStub) ForceFallback(_ context.Context, forced bool) error {
	c.fallback = &forced
	return nil
}
func (c *controllerStub) PreviewAsset(string) (string, bool) { return c.preview, c.preview != "" }

func TestGeneratedMediaPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "segment.mp4")
	if err := os.WriteFile(path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	server := New(&controllerStub{preview: path}, hub, http.NotFoundHandler(), nil, nil, nil, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/ops/media/segment-1", nil))
	if response.Code != http.StatusOK || response.Body.String() != "video" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("status=%d body=%q headers=%v", response.Code, response.Body.String(), response.Header())
	}
}

func TestGenerationConfig(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	config := &configControllerStub{config: GenerationConfig{
		Provider: "MiniMax", BaseURL: "https://api.minimaxi.com", Model: "MiniMax-H3",
		Resolution: "768P", DurationSeconds: 5, Ratio: "16:9", APIKeyConfigured: true,
		UnitPriceCNYPerSecond: 0.5,
	}}
	server := New(&controllerStub{}, hub, http.NotFoundHandler(), nil, config, nil, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/config/generation", strings.NewReader(`{"model":"MiniMax-H3-Max","resolution":"480P","durationSeconds":5,"ratio":"16:9","apiKey":"new-secret"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || config.config.Model != "MiniMax-H3-Max" || config.config.Resolution != "480P" {
		t.Fatalf("status=%d config=%+v", response.Code, config.config)
	}
	if strings.Contains(response.Body.String(), "new-secret") {
		t.Fatal("response exposed API key")
	}
}

func TestBilibiliConfig(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	config := &bilibiliControllerStub{config: BilibiliConfig{Status: "disconnected"}}
	server := New(&controllerStub{}, hub, http.NotFoundHandler(), nil, nil, config, nil)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/config/bilibili", strings.NewReader(`{"roomId":12345,"cookie":"SESSDATA=secret"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || config.config.RoomID != 12345 || !config.config.CookieConfigured {
		t.Fatalf("status=%d config=%+v", response.Code, config.config)
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatal("response exposed Bilibili cookie")
	}
}

func TestInjectMockDanmaku(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	server := New(&controllerStub{}, hub, http.NotFoundHandler(), nil, nil, &bilibiliControllerStub{}, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/config/bilibili/mock", strings.NewReader(`{"username":"本地观众","text":"看看桌上的杯子"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || !strings.Contains(response.Body.String(), "mock-danmaku") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestQwenConfigDoesNotExposeAPIKey(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	config := &qwenControllerStub{config: QwenConfig{ObserverModel: "qwen3.8-flash", DirectorModel: "qwen3.8-max"}}
	server := New(&controllerStub{}, hub, http.NotFoundHandler(), nil, nil, nil, config)
	request := httptest.NewRequest(http.MethodPut, "/api/v1/config/qwen", strings.NewReader(`{"baseUrl":"https://dashscope.aliyuncs.com/compatible-mode/v1","apiKey":"secret"}`))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !config.config.APIKeyConfigured || strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("status=%d body=%s config=%+v", response.Code, response.Body.String(), config.config)
	}
}

func TestCommands(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	controller := &controllerStub{}
	server := New(controller, hub, http.NotFoundHandler(), nil, nil, nil, nil)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/ops/start", strings.NewReader("{}"))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || controller.started != 1 {
		t.Fatalf("start status=%d calls=%d", response.Code, controller.started)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/v1/ops/fallback", strings.NewReader(`{"forced":true}`))
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || controller.fallback == nil || !*controller.fallback {
		t.Fatalf("fallback status=%d forced=%v", response.Code, controller.fallback)
	}
}

func TestFlowReadAPI(t *testing.T) {
	hub := monitoring.NewHub(monitoring.EmptySnapshot(time.Now(), 45*time.Second, 90*time.Second, 30*time.Second))
	server := New(&controllerStub{}, hub, http.NotFoundHandler(), nil, nil, nil, nil)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/flows", nil))
	var list struct {
		Flows []flow.FlowDefinition `json:"flows"`
	}
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || len(list.Flows) != 1 || list.Flows[0].ID != "host-dialogue" {
		t.Fatalf("status=%d flows=%+v", response.Code, list.Flows)
	}

	response = httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/flows/current", nil))
	var current flow.FlowRun
	if err := json.NewDecoder(response.Body).Decode(&current); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || current.Status != flow.RunIdle || current.Direction.HorizonSeconds != 60 || len(current.Beats) != 12 {
		t.Fatalf("status=%d current=%+v", response.Code, current)
	}
}

func TestIsLoopbackAddress(t *testing.T) {
	if !IsLoopbackAddress("127.0.0.1:8080") || IsLoopbackAddress("0.0.0.0:8080") {
		t.Fatal("loopback address validation failed")
	}
}
