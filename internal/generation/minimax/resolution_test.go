package minimax

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streaming-agent/internal/generation"
)

func Test480PInheritsSettingsAndSettlesActualResolution(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/video_generation":
			var body createRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body.Resolution != "480P" || len(body.Content) != 3 {
				t.Errorf("unexpected resolution/content: %s/%d", body.Resolution, len(body.Content))
			}
			_, _ = w.Write([]byte(`{"task_id":"480"}`))
		case "/v2/query/video_generation/480":
			_, _ = w.Write([]byte(`{"task":{"model":"MiniMax-H3-Max","status":"succeeded","resolution":"480P","usage":{"output_seconds":5},"content":{"url":"` + server.URL + `/asset"}}}`))
		case "/asset":
			_, _ = w.Write([]byte("video"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	settings := DefaultSettings()
	if settings.Resolution != "480P" {
		t.Fatal("default resolution must be 480P")
	}
	c, err := New(Config{APIKey: "test", BaseURL: server.URL, DataDir: t.TempDir(), Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	input := generation.Request{Duration: 5 * time.Second, Spec: generation.GenerationSpec{
		Mode: generation.ModeFirstLastFrameToVideo, Prompt: "blink", Duration: 5 * time.Second, Ratio: "adaptive",
		FirstFrame: &generation.AssetRef{URL: "data:first"}, LastFrame: &generation.AssetRef{URL: "data:last"},
	}}
	request, err := c.BuildRequest(input)
	if err != nil {
		t.Fatal(err)
	}
	if request.Spec.Resolution != "480P" || *request.UnitPriceCNY != 0.33 {
		t.Fatal("480P quote not inherited")
	}
	job, err := c.Submit(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	// Settlement uses provider output, even if settings change while the job runs.
	settings.Resolution = "768P"
	if err := c.UpdateSettings(settings); err != nil {
		t.Fatal(err)
	}
	result, err := c.Result(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.CostCNY == nil || math.Abs(*result.CostCNY-1.65) > 1e-9 {
		t.Fatalf("unexpected cost: %v", result.CostCNY)
	}
}
