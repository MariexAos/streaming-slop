package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streaming-agent/internal/config"
	minimaxInference "streaming-agent/internal/inference/minimax"
	httpapi "streaming-agent/internal/server"
)

type configStub struct{ err error }

func (s configStub) GenerationConfig() (httpapi.GenerationConfig, error) {
	return httpapi.GenerationConfig{}, s.err
}
func (s configStub) UpdateGenerationConfig(context.Context, httpapi.GenerationConfigUpdate) (httpapi.GenerationConfig, error) {
	return s.GenerationConfig()
}
func TestSavedCredentialsUpdateDirectorOnlyAfterSuccess(t *testing.T) {
	client := minimaxInference.New("", nil)
	c := liveCredentials{ConfigController: configStub{err: errors.New("save failed")}, inference: client}
	if _, err := c.UpdateGenerationConfig(context.Background(), httpapi.GenerationConfigUpdate{APIKey: "test-key"}); err == nil {
		t.Fatal("expected save error")
	}
	if client.CredentialSaved() {
		t.Fatal("failed save updated director")
	}
	c.ConfigController = configStub{}
	if _, err := c.UpdateGenerationConfig(context.Background(), httpapi.GenerationConfigUpdate{APIKey: "test-key"}); err != nil {
		t.Fatal(err)
	}
	if !client.CredentialSaved() {
		t.Fatal("director missing saved credentials")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestPreviewProxyUsesConfiguredLocalStreamPath(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	var target string
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		target = r.URL.String()
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("playlist"))}, nil
	})
	c := readinessReader{cfg: config.Config{RTMPURL: "rtmp://127.0.0.1:1935/live/test"}}
	response := httptest.NewRecorder()
	c.handler(http.NotFoundHandler()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/live-preview/index.m3u8?test=1", nil))
	if response.Code != 200 || target != "http://127.0.0.1:8888/live/test/index.m3u8?test=1" {
		t.Fatalf("code %d, target %s", response.Code, target)
	}
}

func TestPreviewRedirectRemainsInsideProxy(t *testing.T) {
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	http.DefaultTransport = transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"/live/test/index.m3u8?cookieCheck=1"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	c := readinessReader{cfg: config.Config{RTMPURL: "rtmp://127.0.0.1:1935/live/test"}}
	response := httptest.NewRecorder()
	c.handler(http.NotFoundHandler()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/live-preview/index.m3u8", nil))
	if response.Code != http.StatusFound || response.Header().Get("Location") != "/live-preview/index.m3u8?cookieCheck=1" {
		t.Fatalf("redirect escaped preview: %d %s", response.Code, response.Header().Get("Location"))
	}
}
