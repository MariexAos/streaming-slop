package fal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func TestQueueLifecycleAndImmediateDownload(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asset.mp4" && r.Header.Get("Authorization") != "Key secret" {
			got := r.Header.Get("Authorization")
			t.Errorf("authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/minimax/h3-max/reference-to-video":
			var request submitRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Duration != 5 || request.Resolution != "480P" || request.AspectRatio != "16:9" ||
				request.PromptExpansionMode != "balanced" || !request.EnableSafetyChecker {
				t.Fatalf("unexpected request: %+v", request)
			}
			_, _ = w.Write([]byte(`{"request_id":"job-1","status_url":"status","response_url":"response","cancel_url":"cancel"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"status":"COMPLETED","request_id":"job-1","queue_position":0,"metrics":{"inference_time":12.5}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/requests/job-1"):
			_, _ = w.Write([]byte(`{"video":{"url":"` + server.URL + `/asset.mp4","content_type":"video/mp4","file_name":"clip.mp4","file_size":5},"expanded_prompt":"expanded","seed":7}`))
		case r.Method == http.MethodGet && r.URL.Path == "/asset.mp4":
			_, _ = w.Write([]byte("video"))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/cancel"):
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"CANCELLATION_REQUESTED"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{APIKey: "secret", BaseURL: server.URL, DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.Submit(context.Background(), generation.Request{Direction: live.Direction{
		Action:     "wave",
		Emotion:    "warm",
		Camera:     live.CameraDirection{Shot: "medium", Movement: "locked", Angle: "eye-level"},
		Continuity: live.ContinuityConstraint{Notes: "same room"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.Status(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != generation.JobCompleted {
		t.Fatalf("status = %q", status.Status)
	}
	asset, err := client.Result(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(asset.AssetURL)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "video" || filepath.Ext(asset.AssetURL) != ".mp4" {
		t.Fatalf("download = %q at %q", data, asset.AssetURL)
	}
	if err := client.Cancel(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
}

func TestStatusRejectsUnknownState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"MYSTERY","request_id":"job-1"}`))
	}))
	defer server.Close()
	client, err := New(Config{APIKey: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background(), "job-1"); err == nil {
		t.Fatal("expected unknown status error")
	}
}
