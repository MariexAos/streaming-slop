package store

import (
	"context"
	"errors"

	"streaming-agent/internal/generation"
)

// SessionBudget binds reservations to a session, including late provider results.
type SessionBudget struct {
	store *Store
	id    string
}

func (s *Store) BudgetFor(id string) generation.Budget {
	if id == "" {
		id = "live"
	} // Existing attempts and acceptance tools keep their original ledger.
	return SessionBudget{store: s, id: id}
}
func (b SessionBudget) Reserve(ctx context.Context, id string, amount int64) error {
	return b.store.reserve(ctx, b.id, id, amount)
}
func (b SessionBudget) Settle(ctx context.Context, id string, amount int64) error {
	return b.store.Settle(ctx, id, amount)
}
func (s *Store) ConfigureSessionBudgets(ctx context.Context, limit int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO spending_limits(id,limit_micros) VALUES('next-session',$1) ON CONFLICT DO NOTHING`, limit)
	return err
}
func (s *Store) NextBudget(ctx context.Context) (int64, error) {
	var limit int64
	err := s.pool.QueryRow(ctx, `SELECT limit_micros FROM spending_limits WHERE id='next-session'`).Scan(&limit)
	return limit, err
}
func (s *Store) SaveNextBudget(ctx context.Context, limit int64) error {
	if limit <= 0 {
		return errors.New("本场预算必须大于零")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO spending_limits(id,limit_micros) VALUES('next-session',$1) ON CONFLICT(id) DO UPDATE SET limit_micros=EXCLUDED.limit_micros`, limit)
	return err
}
func (s *Store) CurrentSpending(ctx context.Context) (BudgetState, error) {
	limit, err := s.NextBudget(ctx)
	if err != nil {
		return BudgetState{}, err
	}
	current, err := s.ActiveSession(ctx)
	if err != nil {
		return BudgetState{}, err
	}
	if current == nil {
		return BudgetState{LimitMicros: limit, NextLimitMicros: limit}, nil
	}
	id := string(current.ID)
	if current.BudgetLimitMicros == 0 {
		id = "live"
	}
	state, err := s.spending(ctx, id)
	state.SessionID = string(current.ID)
	state.NextLimitMicros = limit
	return state, err
}
