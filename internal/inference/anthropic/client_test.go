package anthropic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"streaming-agent/internal/director"
)

func TestDirectUsesStructuredOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "secret" {
			t.Errorf("x-api-key = %q", got)
		}
		var request messageRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != DefaultModel {
			t.Errorf("model = %q", request.Model)
		}
		if request.OutputConfig.Format.Type != "json_schema" {
			t.Errorf("format = %q", request.OutputConfig.Format.Type)
		}
		properties := request.OutputConfig.Format.Schema["properties"].(map[string]any)
		camera := properties["camera"].(map[string]any)
		continuity := properties["continuity"].(map[string]any)
		if camera["type"] != "object" || continuity["type"] != "object" {
			t.Fatalf("nested schema = %#v", properties)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_1","type":"message","role":"assistant","model":"test",
			"content":[{"type":"text","text":"{\"action\":\"wave\",\"dialogue\":\"hello\",\"emotion\":\"warm\",\"camera\":{\"shot\":\"medium\",\"movement\":\"locked\",\"angle\":\"eye-level\"},\"continuity\":{\"notes\":\"same room\",\"anchors\":[\"wardrobe\"]}}"}],
			"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"inference_geo":"global","service_tier":"standard"}
		}`))
	}))
	defer server.Close()

	client, err := New(Config{APIKey: "secret", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Direct(context.Background(), director.Input{StorySeed: "a quiet morning"}); err != nil {
		t.Fatal(err)
	}
}

func TestDirectRejectsUnknownDirectionField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{
			"id":"msg_1","type":"message","role":"assistant","model":"test",
			"content":[{"type":"text","text":"{\"action\":\"wave\",\"dialogue\":\"\",\"emotion\":\"warm\",\"camera\":{\"shot\":\"medium\",\"movement\":\"locked\",\"angle\":\"eye-level\"},\"continuity\":{\"notes\":\"same room\",\"anchors\":[]},\"extra\":true}"}],
			"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}
		}`))
	}))
	defer server.Close()

	client, err := New(Config{APIKey: "secret", BaseURL: server.URL, RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Direct(context.Background(), director.Input{}); err == nil {
		t.Fatal("expected strict decode error")
	}
}
