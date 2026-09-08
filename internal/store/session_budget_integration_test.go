//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"

	"github.com/google/uuid"
)

func TestSessionBudgetsIsolateConcurrencyAndLateSettlement(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	first := newBudgetSession(t, s, 5_000_000)
	second := newBudgetSession(t, s, 10_000_000)
	a, b := s.BudgetFor(string(first.ID)), s.BudgetFor(string(second.ID))
	var count atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			err := a.Reserve(ctx, uuid.NewString(), 2_500_000)
			if err == nil {
				count.Add(1)
			} else if !errors.Is(err, generation.ErrBudgetExceeded) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if count.Load() != 2 {
		t.Fatalf("allowed %d reservations beyond first session budget", count.Load())
	}
	video, inference := uuid.NewString(), uuid.NewString()
	if err := b.Reserve(ctx, video, 5_000_000); err != nil {
		t.Fatal(err)
	}
	if err := b.Reserve(ctx, inference, 1_000_000); err != nil {
		t.Fatal(err)
	}
	if err := b.Settle(ctx, inference, 100_000); err != nil {
		t.Fatal(err)
	}
	var oldID string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM spending WHERE budget_id=$1 LIMIT 1`, first.ID).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	if err := a.Settle(ctx, oldID, 200_000); err != nil {
		t.Fatal(err)
	}
	before, err := s.spending(ctx, string(second.ID))
	if err != nil {
		t.Fatal(err)
	}
	if before.ChargedMicros != 100_000 || before.ReservedMicros != 5_000_000 {
		t.Fatalf("late settlement leaked across sessions: %+v", before)
	}
	if err := s.SaveNextBudget(ctx, 20_000_000); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureSessionBudgets(ctx, 10_000_000); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	after, err := restarted.spending(ctx, string(second.ID))
	if err != nil || after != before {
		t.Fatalf("restart or next budget changed active ledger: %+v %v", after, err)
	}
	active, err := restarted.ActiveSession(ctx)
	if err != nil || active == nil || active.ID != second.ID || active.BudgetLimitMicros != 10_000_000 {
		t.Fatalf("session ceiling not recovered: %v", err)
	}
	history, err := restarted.SessionHistory(ctx, string(second.ID))
	if err != nil {
		t.Fatal(err)
	}
	if history.Session.CostCNY != 0.1 || history.Session.ReservedMicros != 5_000_000 || history.Session.BudgetLimitMicros != 10_000_000 {
		t.Fatalf("history omitted inference or pending cost: %+v", history.Session)
	}
}

func newBudgetSession(t *testing.T, s *Store, limit int64) live.LiveSession {
	t.Helper()
	timeline, err := live.NewTimeline(30 * time.Second)
	if err != nil {
		t.Fatal(err)
	}
	value, err := live.NewSession(live.SessionID(uuid.NewString()), live.WorldState{}, timeline, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	value.BudgetLimitMicros = limit
	if err := s.CreateSession(context.Background(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
