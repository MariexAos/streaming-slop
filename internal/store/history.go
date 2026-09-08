package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"streaming-agent/internal/audience"
	"streaming-agent/internal/live"
	"streaming-agent/internal/session"
)

const historySummarySelect = `
	SELECT ls.id::text, ls.status, ls.created_at, ls.updated_at,
		(SELECT count(*) FROM segments s WHERE s.session_id = ls.id),
		(SELECT count(*) FROM assets a WHERE a.session_id = ls.id AND a.source = 'generated'),
		CASE WHEN EXISTS(SELECT 1 FROM spending_limits WHERE id=ls.id::text)
   THEN COALESCE((SELECT sum(charged_micros)::float8/1e6 FROM spending WHERE budget_id=ls.id::text),0)
   ELSE COALESCE((SELECT sum(ga.cost_cny) FROM generation_attempts ga WHERE ga.session_id = ls.id), 0) END,
  COALESCE((SELECT limit_micros FROM spending_limits WHERE id=ls.id::text),0),
  COALESCE((SELECT sum(reserved_micros) FROM spending WHERE budget_id=ls.id::text AND charged_micros IS NULL),0)
	FROM live_sessions ls`

func (s *Store) ListSessionHistory(ctx context.Context) ([]session.HistorySummary, error) {
	rows, err := s.pool.Query(ctx, historySummarySelect+` ORDER BY ls.created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list session history: %w", err)
	}
	defer rows.Close()
	items := make([]session.HistorySummary, 0)
	for rows.Next() {
		var item session.HistorySummary
		if err := rows.Scan(&item.ID, &item.Status, &item.StartedAt, &item.EndedAt, &item.SegmentTotal, &item.ReadyTotal, &item.CostCNY, &item.BudgetLimitMicros, &item.ReservedMicros); err != nil {
			return nil, fmt.Errorf("scan session history: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session history: %w", err)
	}
	return items, nil
}

func (s *Store) SessionHistory(ctx context.Context, id string) (session.History, error) {
	result := session.History{
		Segments: make([]session.HistorySegment, 0), ObserverRuns: make([]session.ObserverRun, 0),
	}
	if err := s.pool.QueryRow(ctx, historySummarySelect+` WHERE ls.id = $1`, id).Scan(
		&result.Session.ID, &result.Session.Status, &result.Session.StartedAt, &result.Session.EndedAt,
		&result.Session.SegmentTotal, &result.Session.ReadyTotal, &result.Session.CostCNY, &result.Session.BudgetLimitMicros, &result.Session.ReservedMicros,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return session.History{}, session.ErrHistoryNotFound
		}
		return session.History{}, fmt.Errorf("read session history: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT s.id::text, s.sequence, s.start_ms, s.end_ms, s.status, s.direction,
			exists(SELECT 1 FROM assets a WHERE a.segment_id = s.id AND a.source = 'generated')
		FROM segments s WHERE s.session_id = $1 ORDER BY s.sequence`, id)
	if err != nil {
		return session.History{}, fmt.Errorf("list history segments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item session.HistorySegment
		var startMS, endMS int64
		var directionJSON []byte
		if err := rows.Scan(&item.ID, &item.Sequence, &startMS, &endMS, &item.Status, &directionJSON, &item.Playable); err != nil {
			return session.History{}, fmt.Errorf("scan history segment: %w", err)
		}
		if err := json.Unmarshal(directionJSON, &item.Direction); err != nil {
			return session.History{}, fmt.Errorf("decode history direction: %w", err)
		}
		item.StartSeconds = float64(startMS) / 1000
		item.EndSeconds = float64(endMS) / 1000
		result.Segments = append(result.Segments, item)
	}
	if err := rows.Err(); err != nil {
		return session.History{}, fmt.Errorf("iterate history segments: %w", err)
	}

	observerRows, err := s.pool.Query(ctx, `
		SELECT audience_revision, messages, observation, error_message, created_at
		FROM observer_runs WHERE session_id = $1 ORDER BY created_at`, id)
	if err != nil {
		return session.History{}, fmt.Errorf("list observer runs: %w", err)
	}
	defer observerRows.Close()
	for observerRows.Next() {
		var item session.ObserverRun
		var messagesJSON, observationJSON []byte
		if err := observerRows.Scan(&item.Revision, &messagesJSON, &observationJSON, &item.Error, &item.CreatedAt); err != nil {
			return session.History{}, fmt.Errorf("scan observer run: %w", err)
		}
		if err := json.Unmarshal(messagesJSON, &item.Messages); err != nil {
			return session.History{}, fmt.Errorf("decode observer messages: %w", err)
		}
		if err := json.Unmarshal(observationJSON, &item.Observation); err != nil {
			return session.History{}, fmt.Errorf("decode observer observation: %w", err)
		}
		result.ObserverRuns = append(result.ObserverRuns, item)
	}
	if err := observerRows.Err(); err != nil {
		return session.History{}, fmt.Errorf("iterate observer runs: %w", err)
	}
	return result, nil
}

func (s *Store) RecordObserverRun(ctx context.Context, sessionID live.SessionID, revision uint64, messages []string, observation audience.Observation, errorMessage string) error {
	messagesJSON, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("encode observer messages: %w", err)
	}
	observationJSON, err := json.Marshal(observation)
	if err != nil {
		return fmt.Errorf("encode observer observation: %w", err)
	}
	var errorValue any
	if errorMessage != "" {
		errorValue = errorMessage
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO observer_runs (session_id, audience_revision, messages, observation, error_message)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (session_id, audience_revision) DO UPDATE
		SET messages = EXCLUDED.messages, observation = EXCLUDED.observation,
			error_message = EXCLUDED.error_message, created_at = now()`,
		sessionID, revision, messagesJSON, observationJSON, errorValue,
	)
	if err != nil {
		return fmt.Errorf("record observer run: %w", err)
	}
	return nil
}

func (s *Store) AssetPath(ctx context.Context, segmentID string) (string, bool, error) {
	var path string
	err := s.pool.QueryRow(ctx, `
		SELECT uri FROM assets
		WHERE segment_id = $1 AND source = 'generated' AND is_current
		ORDER BY created_at DESC LIMIT 1`, segmentID).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read asset path: %w", err)
	}
	return path, true, nil
}
