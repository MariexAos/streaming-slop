package generation

import (
	"errors"
	"fmt"
	"time"

	"streaming-agent/internal/live"
)

const MaxAttemptsPerSegment = 2

type AttemptStatus string

const (
	AttemptPendingSubmit AttemptStatus = "PENDING_SUBMIT"
	AttemptSubmitted     AttemptStatus = "SUBMITTED"
	AttemptRunning       AttemptStatus = "RUNNING"
	AttemptPreparing     AttemptStatus = "PREPARING"
	AttemptSucceeded     AttemptStatus = "SUCCEEDED"
	AttemptFailed        AttemptStatus = "FAILED"
	AttemptCancelled     AttemptStatus = "CANCELLED"
)

type Attempt struct {
	ID             live.AttemptID
	SegmentID      live.SegmentID
	Number         int
	Provider       string
	IdempotencyKey string
	ProviderJobID  string
	Status         AttemptStatus
	ErrorCode      string
	ErrorMessage   string
	Latency        time.Duration
	CostCNY        *float64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SubmittedAt    *time.Time
	StartedAt      *time.Time
	FinishedAt     *time.Time
	Version        int64
}

func NewAttempt(id live.AttemptID, segmentID live.SegmentID, number int, provider, idempotencyKey string, now time.Time) (Attempt, error) {
	if id == "" || segmentID == "" {
		return Attempt{}, errors.New("attempt and segment ids are required")
	}
	if number < 1 || number > MaxAttemptsPerSegment {
		return Attempt{}, fmt.Errorf("attempt number must be between 1 and %d", MaxAttemptsPerSegment)
	}
	if idempotencyKey == "" {
		return Attempt{}, errors.New("idempotency key is required")
	}
	return Attempt{
		ID: id, SegmentID: segmentID, Number: number, Provider: provider,
		IdempotencyKey: idempotencyKey, Status: AttemptPendingSubmit,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (a Attempt) Terminal() bool {
	return a.Status == AttemptSucceeded || a.Status == AttemptFailed || a.Status == AttemptCancelled
}

func (a Attempt) CanRetry() bool {
	return (a.Status == AttemptFailed || a.Status == AttemptCancelled) && a.Number < MaxAttemptsPerSegment
}

func (a *Attempt) Transition(to AttemptStatus, now time.Time) error {
	allowed := map[AttemptStatus]map[AttemptStatus]bool{
		AttemptPendingSubmit: {AttemptSubmitted: true, AttemptFailed: true, AttemptCancelled: true},
		AttemptSubmitted:     {AttemptRunning: true, AttemptFailed: true, AttemptCancelled: true},
		AttemptRunning:       {AttemptPreparing: true, AttemptFailed: true, AttemptCancelled: true},
		AttemptPreparing:     {AttemptSucceeded: true, AttemptFailed: true, AttemptCancelled: true},
	}
	if !allowed[a.Status][to] {
		return fmt.Errorf("attempt transition %s -> %s is not allowed", a.Status, to)
	}
	a.Status = to
	a.UpdatedAt = now
	switch to {
	case AttemptSubmitted:
		a.SubmittedAt = timePointer(now)
	case AttemptRunning:
		a.StartedAt = timePointer(now)
	case AttemptSucceeded, AttemptFailed, AttemptCancelled:
		a.FinishedAt = timePointer(now)
		if a.StartedAt != nil {
			a.Latency = now.Sub(*a.StartedAt)
		}
	}
	return nil
}

func timePointer(value time.Time) *time.Time { return &value }
