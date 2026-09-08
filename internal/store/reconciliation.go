package store

import (
	"context"
	"fmt"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"time"
)

func (s *Store) ClosedAttempts(ctx context.Context) ([]generation.Pending, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT ls.id::text FROM live_sessions ls JOIN generation_attempts a ON a.session_id=ls.id WHERE ls.status IN ('STOPPED','FAILED') AND a.status NOT IN ('SUCCEEDED','FAILED','CANCELLED')`)
	if err != nil {
		return nil, err
	}
	var ids []live.SessionID
	for rows.Next() {
		var id live.SessionID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var result []generation.Pending
	for _, id := range ids {
		attempts, err := s.ListPendingAttempts(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, attempt := range attempts {
			result = append(result, generation.Pending{SessionID: id, Attempt: attempt})
		}
	}
	return result, nil
}
func (s *Store) SettleAttempt(ctx context.Context, id live.SessionID, a generation.Attempt, status generation.JobStatus, cost *float64) error {
	if cost == nil && (a.Provider != "fal" || status != generation.JobCompleted) {
		return fmt.Errorf("missing actual cost")
	}
	// Completion and billing are independent for fal. Keep the reservation
	// until an actual invoice amount is supplied, just as during live playback.
	if cost != nil {
		if err := s.Settle(ctx, string(a.ID), generation.Micros(*cost)); err != nil {
			return err
		}
	}
	a.CostCNY = cost
	now := time.Now().UTC()
	switch status {
	case generation.JobCompleted:
		if a.Status == generation.AttemptSucceeded {
			return s.UpdateAttempt(ctx, id, &a)
		}
		for _, next := range []generation.AttemptStatus{generation.AttemptSubmitted, generation.AttemptRunning, generation.AttemptPreparing, generation.AttemptSucceeded} {
			current := a
			if err := a.Transition(next, now); err != nil {
				continue
			}
			if err := s.UpdateAttempt(ctx, id, &a); err != nil {
				a = current
				return err
			}
		}
	case generation.JobFailed, generation.JobCancelled:
		if err := a.Transition(generation.AttemptCancelled, now); err != nil {
			return err
		}
		return s.UpdateAttempt(ctx, id, &a)
	}
	return nil
}
