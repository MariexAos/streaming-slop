package store

import (
	"context"
	"errors"
	"fmt"

	"streaming-agent/internal/generation"

	"github.com/jackc/pgx/v5"
)

type BudgetState struct {
	LimitMicros    int64 `json:"limitMicros"`
	ChargedMicros  int64 `json:"chargedMicros"`
	ReservedMicros int64 `json:"reservedMicros"`
}

// ConfigureBudget is insert-only: restart cannot reset or raise the ceiling.
func (s *Store) ConfigureBudget(ctx context.Context, limit int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO spending_limits VALUES ('live',$1) ON CONFLICT DO NOTHING`, limit)
	return err
}
func (s *Store) Spending(ctx context.Context) (BudgetState, error) {
	var b BudgetState
	err := s.pool.QueryRow(ctx, `SELECT limit_micros,
 COALESCE((SELECT SUM(charged_micros) FROM spending WHERE budget_id='live'),0),
 COALESCE((SELECT SUM(reserved_micros) FROM spending WHERE budget_id='live' AND charged_micros IS NULL),0)
 FROM spending_limits WHERE id='live'`).Scan(&b.LimitMicros, &b.ChargedMicros, &b.ReservedMicros)
	return b, err
}
func (s *Store) Reserve(ctx context.Context, id string, amount int64) error {
	if id == "" || amount <= 0 {
		return errors.New("reservation requires id and positive amount")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var limit, used int64
	if err = tx.QueryRow(ctx, `SELECT limit_micros FROM spending_limits WHERE id='live' FOR UPDATE`).Scan(&limit); err != nil {
		return fmt.Errorf("lock budget: %w", err)
	}
	var existing int64
	err = tx.QueryRow(ctx, `SELECT reserved_micros FROM spending WHERE id=$1`, id).Scan(&existing)
	if err == nil {
		return errors.New("reservation already exists; reconcile instead of resubmitting")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT COALESCE(SUM(COALESCE(charged_micros,reserved_micros)),0) FROM spending WHERE budget_id='live'`).Scan(&used); err != nil {
		return err
	}
	if amount > limit-used {
		return generation.ErrBudgetExceeded
	}
	if _, err = tx.Exec(ctx, `INSERT INTO spending(id,budget_id,reserved_micros) VALUES($1,'live',$2)`, id, amount); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Settle(ctx context.Context, id string, amount int64) error {
	if amount < 0 {
		return errors.New("negative charge")
	}
	tag, err := s.pool.Exec(ctx, `UPDATE spending SET charged_micros=$2,updated_at=now() WHERE id=$1 AND (charged_micros IS NULL OR charged_micros=$2)`, id, amount)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("charge missing or conflicts with settled amount")
	}
	return nil
}
