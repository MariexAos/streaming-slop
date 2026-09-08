package session

import (
	"context"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"testing"
	"time"
)

type frameStub struct{}

func (frameStub) Frames(context.Context, string) ([]string, error) {
	return []string{"start", "actual-tail"}, nil
}
func TestContinuationRequiresActualPredecessorAndRejectsReplacement(t *testing.T) {
	first, _ := live.NewSegment("first", 0, 0, 5*time.Second, live.Direction{Action: "idle", Emotion: "calm", Camera: live.CameraDirection{Shot: "close", Movement: "fixed"}})
	second := first
	second.ID = "second"
	second.Sequence = 1
	second.Start = 5 * time.Second
	second.End = 10 * time.Second
	r := &Runtime{config: RuntimeConfig{Continuous: true, Frames: frameStub{}}, session: &live.LiveSession{Timeline: live.Timeline{Segments: []live.Segment{first, second}}}}
	if _, err := r.continuation(context.Background(), "second"); err == nil {
		t.Fatal("continued without predecessor")
	}
	r.session.Timeline.Segments[0].Asset = &live.VideoAsset{ID: "old", URI: "clip", Source: live.AssetSourceGenerated}
	link, err := r.continuation(context.Background(), "second")
	if err != nil {
		t.Fatal(err)
	}
	request := generation.Request{ReferenceFrame: &generation.AssetRef{ID: "identity", URL: "original-person"}}
	applyContinuation(&request, link)
	if request.Spec.FirstFrame.URL != "actual-tail" || request.ParentAssetID != "old" {
		t.Fatal("anchor used instead of actual tail")
	}
	if request.Spec.LastFrame == nil || request.Spec.LastFrame.URL != "original-person" || request.Spec.Mode != generation.ModeFirstLastFrameToVideo {
		t.Fatal("continuation lost the original identity anchor")
	}
	r.session.Timeline.Segments[0].Asset = &live.VideoAsset{ID: "new"}
	if r.validContinuation(link) {
		t.Fatal("stale dependency accepted")
	}
}
