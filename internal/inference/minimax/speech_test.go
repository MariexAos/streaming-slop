package minimax

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpeechChargesButRejectsOverlongAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"audio":"616263"},"extra_info":{"audio_length":7000,"usage_characters":4},"base_resp":{"status_code":0}}`))
	}))
	defer server.Close()
	budget := &testBudget{}
	c := New("test", budget)
	c.BaseURL = server.URL
	c.HTTP = server.Client()
	if _, err := c.Synthesize(context.Background(), "hello", "voice", 5*time.Second); err == nil {
		t.Fatal("overlong speech accepted")
	}
	if budget.charged == 0 {
		t.Fatal("successful synthesis not charged")
	}
}
