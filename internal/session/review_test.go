package session

import (
	"context"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"testing"
)

type rejectInspector struct{}

func (rejectInspector) Review(context.Context, generation.Request, []string) (generation.Review, error) {
	return generation.Review{Accepted: false, Reason: "different face", Observed: live.WorldDelta{Events: []live.WorldEvent{{Summary: "untrusted"}}}}, nil
}
func TestRejectedVisualReviewCannotPublishFacts(t *testing.T) {
	r := &Runtime{config: RuntimeConfig{Frames: frameStub{}, Inspector: rejectInspector{}}, store: &memoryStore{}, attempts: []generation.Attempt{{ID: "attempt"}}}
	delta, err := r.inspect(context.Background(), "attempt", "clip")
	if err == nil || len(delta.Events) != 0 {
		t.Fatal("rejected observations escaped review")
	}
}
