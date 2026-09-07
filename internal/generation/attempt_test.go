package generation

import (
	"testing"
	"time"
)

func TestAttemptTransitions(t *testing.T) {
	attempt, err := NewAttempt("attempt-1", "segment-1", 1, "fake", "session:segment:1", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []AttemptStatus{AttemptSubmitted, AttemptRunning, AttemptPreparing, AttemptSucceeded} {
		if err := attempt.Transition(status, attempt.UpdatedAt.Add(time.Second)); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}
	if !attempt.Terminal() {
		t.Fatal("succeeded attempt should be terminal")
	}
}

func TestAttemptLimit(t *testing.T) {
	if _, err := NewAttempt("attempt-3", "segment-1", 3, "fake", "key", time.Now()); err == nil {
		t.Fatal("expected third attempt to be rejected")
	}
}
