package qwen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
)

func TestObserverAndDirectorUseSeparateNonThinkingModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request chatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.EnableThinking || request.ResponseFormat.Type != "json_schema" {
			t.Fatalf("request = %+v", request)
		}
		content := `{"summary":"观众想看杯子","mood":"curious","intents":[{"label":"展示杯子","support":1}]}`
		if request.Model == DefaultDirectorModel {
			content = `{"action":"拿起桌上的杯子简单回应","dialogue":"就这个？我今天刚换的。","emotion":"relaxed","camera":{"shot":"medium close-up","movement":"locked","angle":"eye-level"},"continuity":{"notes":"保持人物和房间不变","anchors":[]}}`
		} else if request.Model != DefaultObserverModel {
			t.Fatalf("model = %q", request.Model)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": content}}}})
	}))
	defer server.Close()

	client := New(Config{APIKey: "test", BaseURL: server.URL, HTTPClient: server.Client()})
	observation, err := client.Observe(context.Background(), audience.Input{Messages: []string{"看看桌上的杯子"}})
	if err != nil || observation.Summary == "" {
		t.Fatalf("observation=%+v err=%v", observation, err)
	}
	directionValue, err := client.Direct(context.Background(), director.Input{Audience: &director.AudienceInput{Summary: observation.Summary}})
	if err != nil || directionValue.Dialogue == "" {
		t.Fatalf("direction=%+v err=%v", directionValue, err)
	}
}
