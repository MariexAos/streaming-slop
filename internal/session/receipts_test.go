package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"streaming-agent/internal/director"
	"streaming-agent/internal/live"
)

type receiptStore struct {
	memoryStore
	failure error
	updates int
}

func (s *receiptStore) UpdateSession(context.Context, *live.LiveSession) error {
	s.updates++
	return s.failure
}

func TestPlaybackReceiptUsesOutputProgressAndCommitsOnce(t *testing.T) {
	store := &receiptStore{}
	output := &fakeOutput{}
	segment, _ := live.NewSegment("one", 0, 0, 5*time.Second, director.IdleDirection())
	segment.Status = live.SegmentPlaying
	runtime := Runtime{store: store, output: output, session: &live.LiveSession{Timeline: live.Timeline{CommitHorizon: 30 * time.Second, Segments: []live.Segment{segment}}}, receipts: []playbackReceipt{{ID: segment.ID, End: segment.End}}}
	runtime.acknowledgePlayback(context.Background())
	if store.updates != 0 || runtime.session.Timeline.Playhead != 0 {
		t.Fatal("pipe input advanced playback before output confirmation")
	}
	output.stats.OutTime = 5 * time.Second
	store.failure = errors.New("database unavailable")
	runtime.acknowledgePlayback(context.Background())
	if runtime.session.Timeline.Playhead != 0 || len(runtime.session.World.Recent) != 0 || len(runtime.receipts) != 1 {
		t.Fatal("failed commit changed the world")
	}
	store.failure = nil
	runtime.acknowledgePlayback(context.Background())
	runtime.acknowledgePlayback(context.Background())
	if store.updates != 2 || runtime.session.Timeline.Playhead != segment.End || len(runtime.session.World.Recent) != 1 {
		t.Fatal("receipt was lost or applied more than once")
	}
}

func TestFallbackReceiptDoesNotApplyStoryChanges(t *testing.T) {
	segment, _ := live.NewSegment("one", 0, 0, 5*time.Second, director.IdleDirection())
	segment.Status = live.SegmentPlaying
	output := &fakeOutput{}
	output.stats.OutTime = segment.End
	runtime := Runtime{store: &receiptStore{}, output: output, session: &live.LiveSession{Timeline: live.Timeline{CommitHorizon: 30 * time.Second, Segments: []live.Segment{segment}}}, receipts: []playbackReceipt{{ID: segment.ID, End: segment.End, Fallback: true}}}
	runtime.acknowledgePlayback(context.Background())
	if runtime.session.Timeline.Playhead != segment.End || len(runtime.session.World.Recent) != 0 {
		t.Fatal("fallback changed story state or failed to advance playback")
	}
}

func TestVisualFactsOnlyCommitWithPlayback(t *testing.T) {
	segment, _ := live.NewSegment("one", 0, 0, 5*time.Second, director.IdleDirection())
	segment.Status = live.SegmentPlaying
	segment.Asset = &live.VideoAsset{Observed: live.WorldDelta{Events: []live.WorldEvent{{Kind: "visual_observation", Summary: "nod visible"}}}}
	output := &fakeOutput{}
	s := &receiptStore{failure: errors.New("commit failed")}
	r := Runtime{store: s, output: output, session: &live.LiveSession{Timeline: live.Timeline{CommitHorizon: 30 * time.Second, Segments: []live.Segment{segment}}}, receipts: []playbackReceipt{{ID: segment.ID, End: segment.End}}}
	r.acknowledgePlayback(context.Background())
	if len(r.session.World.Recent) != 0 {
		t.Fatal("unplayed observation committed")
	}
	output.stats.OutTime = segment.End
	r.acknowledgePlayback(context.Background())
	if len(r.session.World.Recent) != 0 {
		t.Fatal("failed transaction committed observation")
	}
	s.failure = nil
	r.acknowledgePlayback(context.Background())
	r.acknowledgePlayback(context.Background())
	if len(r.session.World.Recent) != 2 || r.session.World.Recent[0].Summary != "nod visible" {
		t.Fatal("observation missing or duplicated")
	}
}
