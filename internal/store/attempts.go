package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func (s *Store) ListAttempts(ctx context.Context, sessionID live.SessionID) ([]generation.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, segment_id::text, attempt_no, provider, idempotency_key,
			provider_job_id, status, error_code, error_message, latency_ms, cost_cny,
			version, created_at, updated_at, submitted_at, started_at, finished_at, plan_revision, request
		FROM generation_attempts
		WHERE session_id = $1
		ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list attempts: %w", err)
	}
	defer rows.Close()
	var attempts []generation.Attempt
	for rows.Next() {
		attempt, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attempts: %w", err)
	}
	return attempts, nil
}

func (s *Store) CreateAttempt(ctx context.Context, sessionID live.SessionID, attempt *generation.Attempt) error {
	if attempt.Status != generation.AttemptPendingSubmit {
		return errors.New("new attempt must be PENDING_SUBMIT")
	}
	if attempt.Number < 1 || attempt.Number > generation.MaxAttemptsPerSegment {
		return errors.New("attempt number is outside the allowed range")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin create attempt: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	segment, err := selectSegment(ctx, tx, sessionID, attempt.SegmentID, true)
	if err != nil {
		return err
	}
	if segment.PlanRevision != attempt.PlanRevision {
		return ErrConflict
	}
	if segment.Status != live.SegmentPlanned && segment.Status != live.SegmentGenerating {
		return fmt.Errorf("segment %s in status %s cannot start generation", segment.ID, segment.Status)
	}
	if err := checkAttemptSlot(ctx, tx, attempt); err != nil {
		return err
	}
	if attempt.Version == 0 {
		attempt.Version = 1
	}
	request, err := json.Marshal(attempt.Request)
	if err != nil {
		return fmt.Errorf("encode generation request: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO generation_attempts (
			id, session_id, segment_id, attempt_no, provider, idempotency_key, status,
			version, created_at, updated_at, plan_revision, request
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		attempt.ID, sessionID, attempt.SegmentID, attempt.Number, attempt.Provider,
		attempt.IdempotencyKey, attempt.Status, attempt.Version, attempt.CreatedAt, attempt.UpdatedAt, attempt.PlanRevision, request,
	)
	if err != nil {
		return fmt.Errorf("insert attempt %s: %w", attempt.ID, err)
	}
	if segment.Status == live.SegmentPlanned {
		if _, err := tx.Exec(ctx, `
			UPDATE segments SET status = 'GENERATING', version = version + 1, updated_at = now()
			WHERE session_id = $1 AND id = $2`, sessionID, segment.ID,
		); err != nil {
			return fmt.Errorf("mark segment generating: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit attempt creation: %w", err)
	}
	return nil
}

func (s *Store) UpdateAttempt(ctx context.Context, sessionID live.SessionID, attempt *generation.Attempt) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin attempt update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	if _, err := selectSegment(ctx, tx, sessionID, attempt.SegmentID, true); err != nil {
		return err
	}
	current, err := selectAttempt(ctx, tx, sessionID, attempt.ID, true)
	if err != nil {
		return err
	}
	if current.Version != attempt.Version {
		return ErrConflict
	}
	if current.Status != attempt.Status {
		if err := current.Transition(attempt.Status, attempt.UpdatedAt); err != nil {
			return err
		}
	}
	tag, err := tx.Exec(ctx, `
		UPDATE generation_attempts
		SET provider_job_id = $3, status = $4, error_code = $5, error_message = $6,
			latency_ms = $7, cost_cny = $8, submitted_at = $9, started_at = $10,
			finished_at = $11, updated_at = $12, version = version + 1
		WHERE session_id = $1 AND id = $2 AND version = $13`,
		sessionID, attempt.ID, nullString(attempt.ProviderJobID), attempt.Status,
		nullString(attempt.ErrorCode), nullString(attempt.ErrorMessage), nullDurationMS(attempt.Latency),
		attempt.CostCNY, attempt.SubmittedAt, attempt.StartedAt, attempt.FinishedAt,
		attempt.UpdatedAt, attempt.Version,
	)
	if err != nil {
		return fmt.Errorf("update attempt %s: %w", attempt.ID, err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit attempt update: %w", err)
	}
	attempt.Version++
	return nil
}

func (s *Store) ListPendingAttempts(ctx context.Context, sessionID live.SessionID) ([]generation.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, segment_id::text, attempt_no, provider, idempotency_key,
			provider_job_id, status, error_code, error_message, latency_ms, cost_cny,
			version, created_at, updated_at, submitted_at, started_at, finished_at, plan_revision, request
		FROM generation_attempts
		WHERE session_id = $1 AND status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED')
		ORDER BY created_at`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list pending attempts: %w", err)
	}
	defer rows.Close()
	var attempts []generation.Attempt
	for rows.Next() {
		attempt, err := scanAttempt(rows)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending attempts: %w", err)
	}
	return attempts, nil
}

func selectAttempt(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, id live.AttemptID, lock bool) (generation.Attempt, error) {
	query := `
		SELECT id::text, segment_id::text, attempt_no, provider, idempotency_key,
			provider_job_id, status, error_code, error_message, latency_ms, cost_cny,
			version, created_at, updated_at, submitted_at, started_at, finished_at, plan_revision, request
		FROM generation_attempts
		WHERE session_id = $1 AND id = $2`
	if lock {
		query += ` FOR UPDATE`
	}
	attempt, err := scanAttempt(tx.QueryRow(ctx, query, sessionID, id))
	if err != nil {
		return generation.Attempt{}, mapNotFound(err, "read attempt")
	}
	return attempt, nil
}

func scanAttempt(row scanner) (generation.Attempt, error) {
	var attempt generation.Attempt
	var request []byte
	var providerJobID, errorCode, errorMessage sql.NullString
	var latencyMS sql.NullInt64
	var cost sql.NullFloat64
	var submittedAt, startedAt, finishedAt sql.NullTime
	if err := row.Scan(
		&attempt.ID, &attempt.SegmentID, &attempt.Number, &attempt.Provider, &attempt.IdempotencyKey,
		&providerJobID, &attempt.Status, &errorCode, &errorMessage, &latencyMS, &cost,
		&attempt.Version, &attempt.CreatedAt, &attempt.UpdatedAt, &submittedAt, &startedAt, &finishedAt, &attempt.PlanRevision, &request,
	); err != nil {
		return generation.Attempt{}, err
	}
	if err := json.Unmarshal(request, &attempt.Request); err != nil {
		return attempt, fmt.Errorf("decode generation request: %w", err)
	}
	attempt.ProviderJobID = providerJobID.String
	attempt.ErrorCode = errorCode.String
	attempt.ErrorMessage = errorMessage.String
	if latencyMS.Valid {
		attempt.Latency = time.Duration(latencyMS.Int64) * time.Millisecond
	}
	if cost.Valid {
		attempt.CostCNY = &cost.Float64
	}
	if submittedAt.Valid {
		attempt.SubmittedAt = &submittedAt.Time
	}
	if startedAt.Valid {
		attempt.StartedAt = &startedAt.Time
	}
	if finishedAt.Valid {
		attempt.FinishedAt = &finishedAt.Time
	}
	return attempt, nil
}

func checkAttemptSlot(ctx context.Context, tx pgx.Tx, attempt *generation.Attempt) error {
	var active bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM generation_attempts
			WHERE segment_id = $1 AND plan_revision = $2 AND status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED')
		)`, attempt.SegmentID, attempt.PlanRevision).Scan(&active); err != nil {
		return fmt.Errorf("check active attempt: %w", err)
	}
	if active {
		return ErrActiveAttempt
	}
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM generation_attempts WHERE segment_id = $1 AND plan_revision = $2`, attempt.SegmentID, attempt.PlanRevision,
	).Scan(&count); err != nil {
		return fmt.Errorf("count segment attempts: %w", err)
	}
	if count >= generation.MaxAttemptsPerSegment || attempt.Number != count+1 {
		return errors.New("attempt number does not follow the segment attempt history")
	}

	return nil
}
