//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"

	"github.com/google/uuid"
)

func TestClosedFalCompletionRetainsReservationUntilInvoice(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tl, _ := live.NewTimeline(30 * time.Second)
	value, _ := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, tl, now)
	if err := s.CreateSession(ctx, &value); err != nil {
		t.Fatal(err)
	}
	segment, _ := live.NewSegment(live.SegmentID(uuid.NewString()), 0, 0, 5*time.Second, testDirection())
	if err := s.UpsertSegment(ctx, value.ID, segment); err != nil {
		t.Fatal(err)
	}
	a, _ := generation.NewAttempt(live.AttemptID(uuid.NewString()), segment.ID, 1, "fal", uuid.NewString(), now)
	if err := s.CreateAttempt(ctx, value.ID, &a); err != nil {
		t.Fatal(err)
	}
	budgetID := uuid.NewString()
	if _, err := s.pool.Exec(ctx, `INSERT INTO spending_limits(id,limit_micros) VALUES($1,1000000)`, budgetID); err != nil {
		t.Fatal(err)
	}
	if err := s.reserve(ctx, budgetID, string(a.ID), 250000); err != nil {
		t.Fatal(err)
	}
	if err := s.SettleAttempt(ctx, value.ID, a, generation.JobCompleted, nil); err != nil {
		t.Fatal(err)
	}
	var status string
	var cost *float64
	if err := s.pool.QueryRow(ctx, `SELECT status,cost_cny FROM generation_attempts WHERE id=$1`, a.ID).Scan(&status, &cost); err != nil {
		t.Fatal(err)
	}
	if status != string(generation.AttemptSucceeded) || cost != nil {
		t.Fatalf("status=%s cost=%v", status, cost)
	}
	b, err := s.spending(ctx, budgetID)
	if err != nil || b.ReservedMicros != 250000 || b.ChargedMicros != 0 {
		t.Fatalf("budget=%+v err=%v", b, err)
	}
	actual := 0.20
	// The same frozen attempt can reconcile an invoice idempotently.
	attempts, err := s.ListAttempts(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := s.SettleAttempt(ctx, value.ID, attempts[0], generation.JobCompleted, &actual); err != nil {
			t.Fatal(err)
		}
		attempts, err = s.ListAttempts(ctx, value.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	b, err = s.spending(ctx, budgetID)
	if err != nil || b.ReservedMicros != 0 || b.ChargedMicros != 200000 {
		t.Fatalf("budget=%+v err=%v", b, err)
	}
}
