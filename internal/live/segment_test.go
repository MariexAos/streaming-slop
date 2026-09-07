package live

import (
	"testing"
	"time"
)

func TestSegmentTransitions(t *testing.T) {
	segment, err := NewSegment("segment-1", 0, 0, 5*time.Second, testDirection())
	if err != nil {
		t.Fatal(err)
	}
	if err := segment.Transition(SegmentGenerating); err != nil {
		t.Fatal(err)
	}
	if err := segment.Transition(SegmentReady); err == nil {
		t.Fatal("expected READY without an asset to fail")
	}
	if err := segment.MarkReady(testAsset()); err != nil {
		t.Fatal(err)
	}
	for _, status := range []SegmentStatus{SegmentCommitted, SegmentPlaying, SegmentPlayed} {
		if err := segment.Transition(status); err != nil {
			t.Fatalf("transition to %s: %v", status, err)
		}
	}
	if err := segment.Transition(SegmentReady); err == nil {
		t.Fatal("expected terminal segment transition to fail")
	}
}

func TestSegmentReplanHonorsCommitHorizon(t *testing.T) {
	segment, err := NewSegment("segment-1", 6, 30*time.Second, 35*time.Second, testDirection())
	if err != nil {
		t.Fatal(err)
	}
	if err := segment.Transition(SegmentGenerating); err != nil {
		t.Fatal(err)
	}
	if err := segment.Replan(testDirection(), 31*time.Second); err == nil {
		t.Fatal("expected replan inside commit horizon to fail")
	}
	if err := segment.Replan(testDirection(), 30*time.Second); err != nil {
		t.Fatalf("replan outside commit horizon: %v", err)
	}
}

func testDirection() Direction {
	return Direction{
		Action: "waits by the window", Emotion: "calm",
		Camera: CameraDirection{Shot: "medium"},
	}
}

func testAsset() VideoAsset {
	return VideoAsset{
		ID: "asset-1", Source: AssetSourceGenerated, NormalizedURI: "/tmp/asset.ts",
		Duration: 5 * time.Second, VerifiedAt: time.Unix(1, 0),
	}
}
