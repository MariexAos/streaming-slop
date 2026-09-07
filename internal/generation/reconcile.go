package generation

import (
	"context"
	"fmt"
	"streaming-agent/internal/live"
)

type Pending struct {
	SessionID live.SessionID
	Attempt   Attempt
}
type ReconciliationStore interface {
	ClosedAttempts(context.Context) ([]Pending, error)
	SettleAttempt(context.Context, live.SessionID, Attempt, JobStatus, *float64) error
}

// ReconcileClosed continues accounting after playback has stopped.
func ReconcileClosed(ctx context.Context, s ReconciliationStore, c Generator, provider string) error {
	pending, err := s.ClosedAttempts(ctx)
	if err != nil {
		return err
	}
	for _, item := range pending {
		a := item.Attempt
		if a.Provider != provider || a.ProviderJobID == "" {
			continue
		}
		job, err := c.Status(ctx, a.ProviderJobID)
		if err != nil {
			return err
		}
		var cost *float64
		switch job.Status {
		case JobCompleted:
			result, err := c.Result(ctx, a.ProviderJobID)
			if err != nil {
				return err
			}
			cost = result.CostCNY
		case JobFailed, JobCancelled:
			zero := 0.0
			cost = &zero
		default:
			continue
		}
		if err = s.SettleAttempt(ctx, item.SessionID, a, job.Status, cost); err != nil {
			return fmt.Errorf("settle stopped attempt %s: %w", a.ID, err)
		}
	}
	return nil
}
