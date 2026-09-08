package generation

import (
	"context"
	"testing"
)

type settlementBudget struct {
	calls   int
	charged int64
}

func (*settlementBudget) Reserve(context.Context, string, int64) error { return nil }
func (b *settlementBudget) Settle(_ context.Context, _ string, amount int64) error {
	b.calls++
	b.charged = amount
	return nil
}

func TestUnsettledCostKeepsReservationUntilActualCost(t *testing.T) {
	ctx := context.Background()
	ledger := &settlementBudget{}
	producer := Budgeted{Budget: ledger, AllowUnsettledCost: true}
	if err := producer.ReconcileCost(ctx, "fal-attempt", nil); err != nil {
		t.Fatal(err)
	}
	if ledger.calls != 0 {
		t.Fatal("missing actual cost must not release or charge the reservation")
	}
	actual := 0.25
	if err := producer.ReconcileCost(ctx, "fal-attempt", &actual); err != nil {
		t.Fatal(err)
	}
	if ledger.calls != 1 || ledger.charged != 250000 {
		t.Fatalf("unexpected actual settlement: %+v", ledger)
	}
	producer.AllowUnsettledCost = false
	if err := producer.ReconcileCost(ctx, "minimax-attempt", nil); err == nil {
		t.Fatal("missing MiniMax cost should remain an error")
	}
}
