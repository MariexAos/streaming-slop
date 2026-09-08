package fal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"streaming-agent/internal/generation"
)

func TestCancellationCompletionRacePreservesFinalStatus(t *testing.T) {
	for _, tc := range []struct {
		name      string
		code      int
		body      string
		wantError bool
	}{
		{"already completed", 400, `{"status":"ALREADY_COMPLETED"}`, false},
		{"requested", 202, `{"status":"CANCELLATION_REQUESTED"}`, false},
		{"other bad request", 400, `{"status":"INVALID_REQUEST"}`, true},
		{"malformed response", 400, `not json`, true},
		{"server failure", 500, `{"status":"ALREADY_COMPLETED"}`, true},
		{"authentication failure", 401, `{"status":"ALREADY_COMPLETED"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPut {
					w.WriteHeader(tc.code)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				_, _ = w.Write([]byte(`{"status":"COMPLETED","request_id":"job"}`))
			}))
			defer server.Close()
			client, err := New(Config{APIKey: "test", BaseURL: server.URL, DataDir: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Cancel(context.Background(), "job"); (err != nil) != tc.wantError {
				t.Fatalf("cancel error=%v", err)
			}
			if !tc.wantError {
				job, err := client.Status(context.Background(), "job")
				if err != nil || job.Status != generation.JobCompleted {
					t.Fatalf("completion lost: %+v %v", job, err)
				}
			}
		})
	}
}
