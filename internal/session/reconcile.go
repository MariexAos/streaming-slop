package session

import (
	"context"
	"errors"
	"fmt"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"time"
)

type UnresolvedAttempt struct {
	ID       live.AttemptID           `json:"id"`
	Provider string                   `json:"provider"`
	JobID    string                   `json:"jobId"`
	Status   generation.AttemptStatus `json:"status"`
}

func (r *Runtime) Unresolved() []UnresolvedAttempt {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := []UnresolvedAttempt{}
	for _, a := range r.attempts {
		if !a.Terminal() {
			result = append(result, UnresolvedAttempt{a.ID, a.Provider, a.ProviderJobID, a.Status})
		}
	}
	return result
}

// AttachJob links a provider-console task to a submission with an unknown outcome.
func (r *Runtime) AttachJob(ctx context.Context, id live.AttemptID, jobID string) error {
	if jobID == "" {
		return errors.New("provider task id required")
	}
	if _, err := r.generator.Status(ctx, jobID); err != nil {
		return fmt.Errorf("verify provider task: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	a := r.attempt(id)
	if a == nil || a.Status != generation.AttemptPendingSubmit || a.ProviderJobID != "" {
		return errors.New("attempt is not awaiting reconciliation")
	}
	for _, other := range r.attempts {
		if other.ProviderJobID == jobID {
			return errors.New("task already linked")
		}
	}
	next := *a
	next.ProviderJobID = jobID
	if err := next.Transition(generation.AttemptSubmitted, time.Now().UTC()); err != nil {
		return err
	}
	if err := r.store.UpdateAttempt(ctx, r.session.ID, &next); err != nil {
		return err
	}
	*a = next
	return nil
}
