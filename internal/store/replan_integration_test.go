//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"

	"github.com/google/uuid"
)

func TestReplanRejectsLateResultsAndRollsBackFrozenReplacement(t *testing.T) {
	ctx := context.Background()
	storage, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err := storage.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tl, _ := live.NewTimeline(30 * time.Second)
	value, _ := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, tl, now)
	if err := storage.CreateSession(ctx, &value); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		segment, _ := live.NewSegment(live.SegmentID(uuid.NewString()), i, time.Duration(i)*5*time.Second, time.Duration(i+1)*5*time.Second, testDirection())
		if err := storage.UpsertSegment(ctx, value.ID, segment); err != nil {
			t.Fatal(err)
		}
		if err := value.Timeline.Append(segment); err != nil {
			t.Fatal(err)
		}
	}
	old := value.Timeline.Segments[6]
	attempt, _ := generation.NewAttempt(live.AttemptID(uuid.NewString()), old.ID, 1, "test", uuid.NewString(), now)
	attempt.Request = generation.Request{Duration: 5 * time.Second, Direction: old.Direction}
	if err := storage.CreateAttempt(ctx, value.ID, &attempt); err != nil {
		t.Fatal(err)
	}
	for _, status := range []generation.AttemptStatus{generation.AttemptSubmitted, generation.AttemptRunning, generation.AttemptPreparing} {
		if err := attempt.Transition(status, now); err != nil {
			t.Fatal(err)
		}
		if err := storage.UpdateAttempt(ctx, value.ID, &attempt); err != nil {
			t.Fatal(err)
		}
	}
	next := old
	if err := next.Replan(testDirection(), 30*time.Second); err != nil {
		t.Fatal(err)
	}
	frozen := value.Timeline.Segments[0]
	frozen.PlanRevision++
	if err := storage.ReplanSegments(ctx, value.ID, []live.Segment{next, frozen}); err == nil {
		t.Fatal("accepted replacement inside commit horizon")
	}
	var revision int64
	if err := storage.pool.QueryRow(ctx, `SELECT plan_revision FROM segments WHERE id=$1`, old.ID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if revision != 0 {
		t.Fatal("partial replan survived rollback")
	}
	if err := storage.ReplanSegments(ctx, value.ID, []live.Segment{next}); err != nil {
		t.Fatal(err)
	}
	asset := live.VideoAsset{ID: live.AssetID(uuid.NewString()), AttemptID: attempt.ID, Source: live.AssetSourceGenerated, NormalizedURI: "test.ts", Duration: 5 * time.Second, VerifiedAt: now}
	if err := storage.MarkReady(ctx, value.ID, attempt.ID, asset); !errors.Is(err, ErrConflict) {
		t.Fatalf("late completion=%v", err)
	}
	replacement, _ := generation.NewAttempt(live.AttemptID(uuid.NewString()), old.ID, 1, "test", uuid.NewString(), now)
	replacement.PlanRevision = 1
	replacement.Request = generation.Request{Duration: 10 * time.Second, Direction: next.Direction}
	if err := storage.CreateAttempt(ctx, value.ID, &replacement); err != nil {
		t.Fatal(err)
	}
	attempts, err := storage.ListAttempts(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 2 || attempts[1].PlanRevision != 1 || attempts[1].Request.Duration != 10*time.Second {
		t.Fatalf("lost generation history: %+v", attempts)
	}
}
