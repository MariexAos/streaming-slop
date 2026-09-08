package fal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
)

func TestTurboUsesExactFramesAndRootQueueURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/" + TurboModel:
			var input map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["image_url"] != "data:first" || input["end_image_url"] != "data:last" || input["duration"] != float64(10) || input["reference_image_urls"] != nil || input["aspect_ratio"] != nil {
				t.Fatalf("incorrect turbo input: %+v", input)
			}
			_, _ = w.Write([]byte(`{"request_id":"job"}`))
		case "/minimax/h3-max-turbo/requests/job/status":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"IN_PROGRESS","request_id":"job"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := New(Config{APIKey: "test", BaseURL: server.URL, Model: TurboModel})
	if err != nil {
		t.Fatal(err)
	}
	job, err := client.Submit(context.Background(), generation.Request{Direction: director.IdleDirection(), Spec: generation.GenerationSpec{Duration: 10 * time.Second, FirstFrame: &generation.AssetRef{URL: "data:first"}, LastFrame: &generation.AssetRef{URL: "data:last"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Status(context.Background(), job.ID); err != nil {
		t.Fatal(err)
	}
}
