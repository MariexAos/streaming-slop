package minimax

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"streaming-agent/internal/director"
)

const validDirection = `{"action":"nod","dialogue":"hello","emotion":"calm","camera":{"shot":"medium","movement":"none","angle":"fixed"},"continuity":{"notes":"same room","anchors":[]}}`

func TestDirectionCorrectsMalformedResponses(t *testing.T) {
	for name, first := range map[string]string{
		"trailing value":        validDirection + `{}`,
		"schema keyword":        strings.Replace(validDirection, `"action":`, `"additionalProperties":false,"action":`, 1),
		"nested schema keyword": strings.Replace(validDirection, `"shot":`, `"additionalProperties":false,"shot":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			c, budget := directionClient(t, func(body request) string {
				calls++
				if !strings.Contains(body.System, director.Instruction) {
					t.Error("shared direction rules missing")
				}
				if calls == 1 {
					return first
				}
				if len(body.Messages[0].Content) != 2 || !strings.Contains(body.Messages[0].Content[1].Text, "failed validation") {
					t.Error("correction missing")
				}
				return validDirection
			})
			value, err := c.Direct(context.Background(), director.Input{})
			if err != nil || value.Action != "nod" || calls != 2 || budget.charged == 0 {
				t.Fatalf("calls=%d result=%+v err=%v", calls, value, err)
			}
		})
	}
}

func TestDirectionCorrectionIsBounded(t *testing.T) {
	calls := 0
	c, _ := directionClient(t, func(request) string { calls++; return validDirection + `{}` })
	if _, err := c.Direct(context.Background(), director.Input{}); err == nil || calls != 2 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestDirectionAcceptsOnlySingleEnclosingFence(t *testing.T) {
	for _, text := range []string{validDirection, "```json\n" + validDirection + "\n```"} {
		if _, err := director.DecodeDirection(directionJSON([]byte(text))); err != nil {
			t.Fatal(err)
		}
	}
	for _, text := range []string{validDirection + `{}`, "```json\n" + validDirection + `{}` + "\n```", "```json\n" + validDirection + "\n```\nextra"} {
		if _, err := director.DecodeDirection(directionJSON([]byte(text))); err == nil {
			t.Fatal("invalid suffix accepted")
		}
	}
}

func directionClient(t *testing.T, respond func(request) string) (*Client, *testBudget) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "count_tokens") {
			_, _ = w.Write([]byte(`{"input_tokens":100}`))
			return
		}
		var body request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"content": []Part{{Type: "text", Text: respond(body)}}, "usage": map[string]int{"input_tokens": 100, "output_tokens": 100}, "stop_reason": "end_turn"})
	}))
	t.Cleanup(server.Close)
	budget := &testBudget{}
	c := New("test", budget)
	c.BaseURL, c.HTTP = server.URL, server.Client()
	return c, budget
}
