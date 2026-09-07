package session

import (
	"context"
	"streaming-agent/internal/live"
)

type costReconciler interface {
	ReconcileCost(context.Context, string, *float64) error
}

func (r *Runtime) recordCost(ctx context.Context, sessionID live.SessionID, id live.AttemptID, cost *float64) error {
	if ledger, ok := r.generator.(costReconciler); ok {
		if err := ledger.ReconcileCost(ctx, string(id), cost); err != nil {
			return err
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if attempt := r.attempt(id); attempt != nil {
		attempt.CostCNY = cost
		return r.store.UpdateAttempt(ctx, sessionID, attempt)
	}
	return nil
}
