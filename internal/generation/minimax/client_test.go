package minimax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func TestLifecycleAndCost(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/asset.mp4" && r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v2/video_generation":
			var request createRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatal(err)
			}
			if request.Model != "MiniMax-H3-Max" || request.Resolution != "768P" || request.Duration != 5 || request.Ratio != "16:9" {
				t.Fatalf("request = %+v", request)
			}
			_, _ = w.Write([]byte(`{"task_id":"job-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/query/video_generation/job-1":
			_, _ = w.Write([]byte(`{"task":{"id":"job-1","model":"MiniMax-H3-Max","status":"succeeded","resolution":"768P","duration":5,"usage":{"output_seconds":5},"content":{"url":"` + server.URL + `/asset.mp4"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/asset.mp4":
			_, _ = w.Write([]byte("video"))
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	settings := DefaultSettings()
	settings.Resolution = "768P"
	client, err := New(Config{APIKey: "secret", BaseURL: server.URL, DataDir: t.TempDir(), Settings: settings})
	if err != nil {
		t.Fatal(err)
	}
	request := generation.Request{Direction: live.Direction{
		Action: "wave", Emotion: "warm",
		Camera:     live.CameraDirection{Shot: "medium", Movement: "locked", Angle: "eye-level"},
		Continuity: live.ContinuityConstraint{Notes: "same room"},
	}}
	job, err := client.Submit(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.Status(context.Background(), job.ID)
	if err != nil || status.Status != generation.JobCompleted {
		t.Fatalf("status=%+v error=%v", status, err)
	}
	result, err := client.Result(context.Background(), job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.CostCNY == nil || *result.CostCNY != 2.5 {
		t.Fatalf("cost = %v", result.CostCNY)
	}
	if _, err := os.Stat(result.AssetURL); err != nil {
		t.Fatal(err)
	}
}

func TestPricingAndSettings(t *testing.T) {
	price, err := UnitPriceCNY("MiniMax-H3-Max", "768P")
	if err != nil || price != 0.50 {
		t.Fatalf("price=%v error=%v", price, err)
	}
	invalid := []Settings{
		{Model: "MiniMax-H3", Resolution: "768P", Duration: 5, Ratio: "16:9"},
		{Model: "MiniMax-H3-Max", Resolution: "2K", Duration: 5, Ratio: "16:9"},
		{Model: "MiniMax-H3-Max", Resolution: "720P", Duration: 5, Ratio: "16:9"},
	}
	for _, settings := range invalid {
		if err := settings.Validate(); err == nil {
			t.Fatalf("expected invalid settings: %+v", settings)
		}
	}
}

func TestContentRolesForOnlineModes(t *testing.T) {
	first := &generation.AssetRef{URL: "https://example.com/first.png"}
	last := &generation.AssetRef{URL: "https://example.com/last.png"}
	tests := []struct {
		name  string
		spec  generation.GenerationSpec
		roles []string
	}{
		{"text", generation.GenerationSpec{Mode: generation.ModeTextToVideo, Prompt: "wide shot", Resolution: "768P", Duration: 5 * time.Second, Ratio: "16:9"}, []string{""}},
		{"first", generation.GenerationSpec{Mode: generation.ModeFirstFrameToVideo, Prompt: "talk", FirstFrame: first, Resolution: "768P", Duration: 5 * time.Second, Ratio: "adaptive"}, []string{"", "first_frame"}},
		{"first last", generation.GenerationSpec{Mode: generation.ModeFirstLastFrameToVideo, Prompt: "turn", FirstFrame: first, LastFrame: last, Resolution: "768P", Duration: 5 * time.Second, Ratio: "adaptive"}, []string{"", "first_frame", "last_frame"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := contentFor(test.spec)
			if len(items) != len(test.roles) {
				t.Fatalf("content count = %d, want %d", len(items), len(test.roles))
			}
			for i, role := range test.roles {
				if items[i].Role != role {
					t.Fatalf("content[%d].role = %q, want %q", i, items[i].Role, role)
				}
			}
		})
	}
}

func TestAPIKeyCanBeConfiguredAfterStartup(t *testing.T) {
	client, err := New(Config{Settings: DefaultSettings()})
	if err != nil {
		t.Fatal(err)
	}
	if client.APIKeyConfigured() {
		t.Fatal("API key should not be configured")
	}
	if err := client.UpdateAPIKey(" secret "); err != nil {
		t.Fatal(err)
	}
	if !client.APIKeyConfigured() {
		t.Fatal("API key should be configured")
	}
}

func TestRejectsMultimodalReferences(t *testing.T) {
	client, err := New(Config{APIKey: "secret", Settings: DefaultSettings()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Submit(context.Background(), generation.Request{References: []string{"https://example.com/reference.png"}})
	if err == nil || !strings.Contains(err.Error(), "multimodal references") {
		t.Fatalf("Submit() error = %v", err)
	}
}
