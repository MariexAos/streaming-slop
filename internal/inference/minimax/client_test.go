package minimax

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"streaming-agent/internal/generation"
	"strings"
	"testing"
)

type testBudget struct {
	fail              bool
	reserved, charged int64
}

func (b *testBudget) Reserve(_ context.Context, _ string, n int64) error {
	if b.fail {
		return generation.ErrBudgetExceeded
	}
	b.reserved = n
	return nil
}
func (b *testBudget) Settle(_ context.Context, _ string, n int64) error { b.charged = n; return nil }
func TestM3BudgetBeforeGenerationAndChargeMalformedOutput(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("auth missing")
		}
		if strings.HasSuffix(r.URL.Path, "count_tokens") {
			_, _ = w.Write([]byte(`{"input_tokens":100}`))
			return
		}
		calls++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["model"] != "MiniMax-M3" || request["max_tokens"] != float64(1024) {
			t.Error("incorrect model/output limit")
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"invalid json"}],"usage":{"input_tokens":100,"output_tokens":10},"stop_reason":"end_turn"}`))
	}))
	defer server.Close()
	budget := &testBudget{fail: true}
	c := New("test", budget)
	c.BaseURL = server.URL
	c.HTTP = server.Client()
	_, err := c.complete(context.Background(), "test", []Part{{Type: "text", Text: "hello"}})
	if !errors.Is(err, generation.ErrBudgetExceeded) || calls != 0 {
		t.Fatal("provider called without budget")
	}
	budget.fail = false
	_, err = c.Review(context.Background(), generation.Request{}, nil)
	if err == nil || budget.charged == 0 || budget.reserved < budget.charged {
		t.Fatal("malformed model output not charged")
	}
}
