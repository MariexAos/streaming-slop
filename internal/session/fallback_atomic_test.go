package session

import (
	"context"
	"errors"
	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
	"testing"
	"time"
)

type fallbackStore struct {
	memoryStore
	runtime *Runtime
	failure error
}

func (s *fallbackStore) MarkFallbackReady(_ context.Context, _ live.SessionID, _ live.SegmentID, _ live.VideoAsset) error {
	if s.runtime.session.Timeline.Segments[0].Status != live.SegmentGenerating {
		return errors.New("published before persistence")
	}
	return s.failure
}
func TestFallbackPublishedOnlyAfterPersistence(t *testing.T) {
	segment, _ := live.NewSegment("one", 0, 0, 5*time.Second, director.IdleDirection())
	store := &fallbackStore{failure: errors.New("database unavailable")}
	r := &Runtime{store: store, session: &live.LiveSession{ID: "session", Timeline: live.Timeline{Segments: []live.Segment{segment}}}}
	r.fallback.Path = "fallback.ts"
	r.fallback.Duration = 5 * time.Second
	store.runtime = r
	r.ensureFallbackSegmentLocked(context.Background())
	if r.session.Timeline.Segments[0].Asset != nil {
		t.Fatal("failed persistence exposed fallback")
	}
	store.failure = nil
	r.ensureFallbackSegmentLocked(context.Background())
	if r.session.Timeline.Segments[0].Status != live.SegmentReady {
		t.Fatal("saved fallback not ready")
	}
}
