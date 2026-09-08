package session

import (
	"context"
	"testing"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func TestCompletedVideoBecomesReadyWithoutVisualReview(t *testing.T) {
	segment, err := live.NewSegment("segment", 0, 0, 5*time.Second, director.IdleDirection())
	if err != nil {
		t.Fatal(err)
	}
	segment.Status = live.SegmentGenerating
	attempt, err := generation.NewAttempt("attempt", segment.ID, 1, "fal", "request", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	attempt.Status = generation.AttemptSubmitted
	attempt.PlanRevision = segment.PlanRevision
	attempt.Request.Duration = 5 * time.Second
	runtime := Runtime{store: &memoryStore{}, generator: fakeGenerator{}, preparer: &fakePreparer{},
		session:  &live.LiveSession{ID: "session", Timeline: live.Timeline{Segments: []live.Segment{segment}}},
		attempts: []generation.Attempt{attempt},
	}
	runtime.completeAttempt(context.Background(), "session", "attempt", "job")
	result := runtime.session.Timeline.Segments[0]
	if result.Status != live.SegmentReady || result.Asset == nil || runtime.attempts[0].Status != generation.AttemptSucceeded {
		t.Fatalf("completed video did not become playable: status=%s, error=%s", result.Status, runtime.lastGenErr)
	}
	if len(result.Asset.Observed.Events) != 0 {
		t.Fatal("unobserved video must not invent visual facts")
	}
}
