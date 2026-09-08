package session

import (
	"context"
	"os"
	"path/filepath"
	"streaming-agent/internal/director"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"testing"
	"time"
)

func TestReuseRequiresMatchingGenerationInputs(t *testing.T) {
	original := generation.Request{Provider: "fal", Model: "turbo", BudgetID: "old", Spec: generation.GenerationSpec{Prompt: "nod", Resolution: "480P", FirstFrame: &generation.AssetRef{URL: "first"}}}
	request := original
	request.BudgetID, request.IdempotencyKey = "new", "new-attempt"
	if reuseKey(request) != reuseKey(original) {
		t.Fatal("accounting prevented reuse")
	}
	request.Spec.FirstFrame = &generation.AssetRef{URL: "different"}
	if reuseKey(request) == reuseKey(original) {
		t.Fatal("different starting frame reused")
	}
	request = original
	request.Model = "other"
	if reuseKey(request) == reuseKey(original) {
		t.Fatal("different model reused")
	}
}

func TestReuseCompletesWithoutProviderCallOrNewCost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clip.ts")
	if err := os.WriteFile(path, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	old, _ := live.NewSegment("old", 0, 0, 5*time.Second, director.IdleDirection())
	old.Status = live.SegmentPlayed
	old.Asset = &live.VideoAsset{ID: "asset", AttemptID: "old-attempt", Source: live.AssetSourceGenerated, NormalizedURI: path, Duration: 5 * time.Second, VerifiedAt: time.Now()}
	next, _ := live.NewSegment("new", 1, 5*time.Second, 10*time.Second, director.IdleDirection())
	next.Status = live.SegmentGenerating
	request := generation.Request{AttemptID: "new-attempt", SegmentID: "new", Provider: "fal", Model: "turbo"}
	previous := generation.Attempt{ID: "old-attempt", SegmentID: "old", Status: generation.AttemptSucceeded, Request: request}
	attempt, _ := generation.NewAttempt("new-attempt", "new", 1, "fal", "key", time.Now())
	attempt.Request, attempt.PlanRevision = request, next.PlanRevision
	r := Runtime{store: &memoryStore{}, session: &live.LiveSession{ID: "session", Timeline: live.Timeline{Segments: []live.Segment{old, next}}}, attempts: []generation.Attempt{previous, attempt}}
	r.submitAttempt(context.Background(), "session", attempt, request)
	if r.attempts[1].Status != generation.AttemptSucceeded || r.attempts[1].CostCNY == nil || *r.attempts[1].CostCNY != 0 {
		t.Fatal("reuse did not complete at zero new cost")
	}
	if r.session.Timeline.Segments[1].Asset.NormalizedURI != path {
		t.Fatal("existing media was not reused")
	}
}
