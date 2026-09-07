//go:build integration

package store

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestBudgetConcurrentReservationsAndRestart(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.pool.Exec(ctx, `TRUNCATE spending_limits CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err = s.ConfigureBudget(ctx, 5_000_000); err != nil {
		t.Fatal(err)
	}
	var count atomic.Int32
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Go(func() {
			if s.Reserve(ctx, string(rune('a'+i)), 2_500_000) == nil {
				count.Add(1)
			}
		})
	}
	wg.Wait()
	if count.Load() != 2 {
		t.Fatalf("allowed %d reservations", count.Load())
	}
	if err = s.ConfigureBudget(ctx, 100_000_000); err != nil {
		t.Fatal(err)
	}
	b, err := s.Spending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if b.LimitMicros != 5_000_000 || b.ReservedMicros != 5_000_000 {
		t.Fatalf("budget reset: %+v", b)
	}
	var id string
	if err = s.pool.QueryRow(ctx, `SELECT id FROM spending LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, id, 2_500_000); err != nil {
		t.Fatal(err)
	}
	if err = s.Settle(ctx, id, 2_500_000); err != nil {
		t.Fatal(err)
	}
	if err = s.Reserve(ctx, id, 2_500_000); err == nil {
		t.Fatal("duplicate allowed")
	}
	if err = s.Settle(ctx, id, 0); err == nil {
		t.Fatal("settled charge rewritten")
	}
}
