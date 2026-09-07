package session

import (
	"context"
	"errors"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"testing"
	"time"
)

type reconcileStore struct {
	memoryStore
	failure error
}

func (s *reconcileStore) UpdateAttempt(context.Context, live.SessionID, *generation.Attempt) error {
	return s.failure
}
func TestAttachUnknownJobPersistsBeforeChangingMemory(t *testing.T) {
	s := &reconcileStore{failure: errors.New("database offline")}
	a, _ := generation.NewAttempt("attempt", "segment", 1, "minimax", "key", time.Now())
	r := &Runtime{store: s, generator: fakeGenerator{}, session: &live.LiveSession{ID: "session"}, attempts: []generation.Attempt{a}}
	if err := r.AttachJob(context.Background(), a.ID, "job"); err == nil {
		t.Fatal("binding ignored persistence failure")
	}
	if r.attempts[0].ProviderJobID != "" {
		t.Fatal("memory linked without commit")
	}
	s.failure = nil
	if err := r.AttachJob(context.Background(), a.ID, "job"); err != nil {
		t.Fatal(err)
	}
	if r.attempts[0].Status != generation.AttemptSubmitted {
		t.Fatal("not submitted")
	}
	if err := r.AttachJob(context.Background(), a.ID, "other-job"); err == nil {
		t.Fatal("existing binding replaced")
	}
}
func TestFallbackDoesNotPretendRunningTaskWasCancelled(t *testing.T) {
	a, _ := generation.NewAttempt("attempt", "segment", 1, "minimax", "key", time.Now())
	a.Status = generation.AttemptRunning
	a.ProviderJobID = "running-job"
	r := &Runtime{attempts: []generation.Attempt{a}}
	pending := r.cancelSegmentAttemptsLocked(a.SegmentID)
	if len(pending) != 1 || r.attempts[0].Terminal() {
		t.Fatal("running task stopped being polled")
	}
}
