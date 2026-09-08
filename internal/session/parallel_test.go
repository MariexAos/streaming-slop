package session

import (
	"context"
	"testing"
	"time"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

type blockedGenerator struct {
	fakeGenerator
	entered chan struct{}
	release chan struct{}
}

func (g blockedGenerator) Submit(ctx context.Context, r generation.Request) (generation.Job, error) {
	g.entered <- struct{}{}
	select {
	case <-g.release:
		return generation.Job{ID: string(r.AttemptID), Status: generation.JobQueued}, nil
	case <-ctx.Done():
		return generation.Job{}, ctx.Err()
	}
}
func TestAnchoredGenerationSubmitsConcurrentlyWithinLimit(t *testing.T) {
	g := blockedGenerator{entered: make(chan struct{}, 3), release: make(chan struct{})}
	defer close(g.release)
	var segments []live.Segment
	for i, id := range []live.SegmentID{"one", "two", "three"} {
		segment, _ := live.NewSegment(id, i, time.Duration(i)*5*time.Second, time.Duration(i+1)*5*time.Second, director.IdleDirection())
		segments = append(segments, segment)
	}
	r := Runtime{config: RuntimeConfig{Provider: "fal", SegmentDuration: 5 * time.Second}, store: &memoryStore{}, generator: g, scheduler: generation.NewScheduler(generation.DefaultSchedulerConfig(2)), session: &live.LiveSession{ID: "session", Timeline: live.Timeline{Segments: segments}}}
	done := make(chan struct{})
	go func() { r.submit(t.Context()); close(done) }()
	for range 2 {
		select {
		case <-g.entered:
		case <-time.After(time.Second):
			t.Fatal("submissions serialized or waiting for predecessor")
		}
	}
	select {
	case <-g.entered:
		t.Fatal("concurrency limit exceeded")
	default:
	}
	g.release <- struct{}{}
	g.release <- struct{}{}
	<-done
}

type forbiddenObserver struct{}

func (forbiddenObserver) Observe(context.Context, audience.Input) (audience.Observation, error) {
	panic("observer model must not block text interaction")
}
func TestTextInteractionReachesDirectorWithoutObserver(t *testing.T) {
	r := Runtime{config: RuntimeConfig{Observer: forbiddenObserver{}}, store: &memoryStore{}}
	snapshot := audience.Snapshot{Revision: 1, Messages: []string{"把纸飞机飞向镜头"}, Summary: "把纸飞机飞向镜头", MessageCount: 1}
	observation, ok := r.observeAudience(t.Context(), "session", snapshot)
	if !ok || observation.Summary != snapshot.Summary {
		t.Fatal("original steering request was lost")
	}
	input := observedAudience(snapshot, observation)
	if len(input.Messages) != 1 || input.Messages[0] != snapshot.Messages[0] {
		t.Fatal("director lost raw message")
	}
}
