//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func TestStoreMigrationRecoveryAndAtomicReady(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	store, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.pool.Exec(ctx, "TRUNCATE live_sessions CASCADE"); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	timelineValue, err := live.NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	sessionValue, err := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, timelineValue, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSession(ctx, &sessionValue); err != nil {
		t.Fatal(err)
	}

	segments := make([]live.Segment, 5)
	for i := range segments {
		segments[i], err = live.NewSegment(
			live.SegmentID(uuid.NewString()), i, time.Duration(i)*5*time.Second,
			time.Duration(i+1)*5*time.Second, testDirection(),
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := store.UpsertSegments(ctx, sessionValue.ID, segments); err != nil {
		t.Fatal(err)
	}

	wanted := []generation.AttemptStatus{
		generation.AttemptPendingSubmit,
		generation.AttemptSubmitted,
		generation.AttemptRunning,
		generation.AttemptPreparing,
	}
	attempts := make([]generation.Attempt, len(wanted))
	for i, status := range wanted {
		attempts[i], err = generation.NewAttempt(
			live.AttemptID(uuid.NewString()), segments[i].ID, 1, "fal", uuid.NewString(), now,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateAttempt(ctx, sessionValue.ID, &attempts[i]); err != nil {
			t.Fatal(err)
		}
		attempts[i].ProviderJobID = "job-" + uuid.NewString()
		for attempts[i].Status != status {
			next := nextAttemptStatus(attempts[i].Status)
			if err := attempts[i].Transition(next, now.Add(time.Duration(i+1)*time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := store.UpdateAttempt(ctx, sessionValue.ID, &attempts[i]); err != nil {
				t.Fatal(err)
			}
		}
	}

	pending, err := store.ListPendingAttempts(ctx, sessionValue.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 4 {
		t.Fatalf("pending attempts=%d, want 4", len(pending))
	}
	for i := range pending {
		if pending[i].Status != wanted[i] {
			t.Fatalf("pending[%d]=%s, want %s", i, pending[i].Status, wanted[i])
		}
	}

	asset := live.VideoAsset{
		ID: live.AssetID(uuid.NewString()), AttemptID: attempts[3].ID,
		Source: live.AssetSourceGenerated, URI: "/tmp/source.mp4", NormalizedURI: "/tmp/segment.ts",
		Duration: 5 * time.Second, Width: 1280, Height: 720, FPS: 30, VerifiedAt: now.Add(10 * time.Second),
	}
	if err := store.MarkReady(ctx, sessionValue.ID, attempts[3].ID, asset); err != nil {
		t.Fatal(err)
	}
	active, err := store.ActiveSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ready, ok := active.Timeline.Segment(segments[3].ID)
	if !ok || ready.Status != live.SegmentReady || ready.Asset == nil || ready.Asset.ID != asset.ID {
		t.Fatalf("ready segment was not committed atomically: %#v", ready)
	}

	// A completion increments the stored version; an old response must not overwrite it.
	if err := store.UpdateAttempt(ctx, sessionValue.ID, &attempts[3]); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale attempt update = %v, want version conflict", err)
	}
	if err := ready.Transition(live.SegmentCommitted); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSegment(ctx, sessionValue.ID, *ready); err != nil {
		t.Fatal(err)
	}
	replacement := asset
	replacement.ID = live.AssetID(uuid.NewString())
	if err := store.MarkReady(ctx, sessionValue.ID, attempts[3].ID, replacement); err == nil {
		t.Fatal("late result replaced a committed segment")
	}
	recovered, err := store.ActiveSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	committed, ok := recovered.Timeline.Segment(ready.ID)
	if !ok || committed.Status != live.SegmentCommitted || committed.Asset == nil || committed.Asset.ID != asset.ID {
		t.Fatalf("committed segment changed after rejected result: %+v", committed)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			attempt, err := generation.NewAttempt(
				live.AttemptID(uuid.NewString()), segments[4].ID, 1, "fal", uuid.NewString(), now,
			)
			if err == nil {
				err = store.CreateAttempt(ctx, sessionValue.ID, &attempt)
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent reservation successes=%d, want 1", successes)
	}
}

func nextAttemptStatus(status generation.AttemptStatus) generation.AttemptStatus {
	switch status {
	case generation.AttemptPendingSubmit:
		return generation.AttemptSubmitted
	case generation.AttemptSubmitted:
		return generation.AttemptRunning
	default:
		return generation.AttemptPreparing
	}
}

func testDirection() live.Direction {
	return live.Direction{
		Action: "wait", Emotion: "calm",
		Camera:     live.CameraDirection{Shot: "medium", Movement: "locked", Angle: "eye-level"},
		Continuity: live.ContinuityConstraint{Notes: "continue"},
	}
}
