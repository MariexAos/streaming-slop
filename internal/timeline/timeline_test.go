package timeline

import (
	"fmt"
	"testing"
	"time"

	"streaming-agent/internal/live"
)

func TestAheadStopsAtFirstUncoveredSegment(t *testing.T) {
	timeline := makeTimeline(t, 4)
	markReady(t, &timeline.Segments[0])
	timeline.Segments[1].Status = live.SegmentGenerating
	markReady(t, &timeline.Segments[2])

	if got := ReadyAhead(timeline); got != 5*time.Second {
		t.Fatalf("ReadyAhead = %s, want 5s", got)
	}
	if got := SubmittedAhead(timeline); got != 15*time.Second {
		t.Fatalf("SubmittedAhead = %s, want 15s", got)
	}
}

func TestAheadUsesRemainingCurrentSegment(t *testing.T) {
	timeline := makeTimeline(t, 2)
	markReady(t, &timeline.Segments[0])
	markReady(t, &timeline.Segments[1])
	timeline.Playhead = 2 * time.Second

	if got := ReadyAhead(timeline); got != 8*time.Second {
		t.Fatalf("ReadyAhead = %s, want 8s", got)
	}
}

func TestCommitAndPlaybackAreOrdered(t *testing.T) {
	timeline := makeTimeline(t, 8)
	for i := range timeline.Segments {
		markReady(t, &timeline.Segments[i])
	}
	committed, err := CommitReady(&timeline)
	if err != nil {
		t.Fatal(err)
	}
	if len(committed) != 6 {
		t.Fatalf("committed %d segments, want 6", len(committed))
	}
	playing, err := StartNext(&timeline)
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishPlaying(&timeline, playing.ID); err != nil {
		t.Fatal(err)
	}
	if timeline.Playhead != 5*time.Second {
		t.Fatalf("playhead = %s, want 5s", timeline.Playhead)
	}
}

func makeTimeline(t *testing.T, count int) live.Timeline {
	t.Helper()
	timeline, err := live.NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		segment, err := live.NewSegment(
			live.SegmentID(fmt.Sprintf("segment-%d", i)), i,
			time.Duration(i)*5*time.Second, time.Duration(i+1)*5*time.Second,
			direction(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := timeline.Append(segment); err != nil {
			t.Fatal(err)
		}
	}
	return timeline
}

func markReady(t *testing.T, segment *live.Segment) {
	t.Helper()
	if err := segment.Transition(live.SegmentGenerating); err != nil {
		t.Fatal(err)
	}
	asset := live.VideoAsset{
		ID: live.AssetID("asset-" + segment.ID), Source: live.AssetSourceGenerated,
		NormalizedURI: "/tmp/" + string(segment.ID) + ".ts", Duration: 5 * time.Second,
		VerifiedAt: time.Unix(1, 0),
	}
	if err := segment.MarkReady(asset); err != nil {
		t.Fatal(err)
	}
}

func direction() live.Direction {
	return live.Direction{Action: "waits", Emotion: "calm", Camera: live.CameraDirection{Shot: "medium"}}
}
