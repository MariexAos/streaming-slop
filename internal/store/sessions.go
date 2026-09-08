package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
)

type runtimeConfig struct {
	BudgetLimitMicros int64                  `json:"budgetLimitMicros,omitempty"`
	Models            *live.ModelSettings    `json:"models,omitempty"`
	Profile           *live.CharacterProfile `json:"profile,omitempty"`
	CommitHorizonMS   int64                  `json:"commitHorizonMs"`
}

func (s *Store) CreateSession(ctx context.Context, session *live.LiveSession) error {
	if err := session.Timeline.Validate(); err != nil {
		return fmt.Errorf("validate session timeline: %w", err)
	}
	world, err := json.Marshal(session.World)
	if err != nil {
		return fmt.Errorf("encode world state: %w", err)
	}
	stream, err := json.Marshal(session.Stream)
	if err != nil {
		return fmt.Errorf("encode stream state: %w", err)
	}
	config, err := json.Marshal(runtimeConfig{BudgetLimitMicros: session.BudgetLimitMicros, Models: session.Models, Profile: session.Profile, CommitHorizonMS: session.Timeline.CommitHorizon.Milliseconds()})
	if err != nil {
		return fmt.Errorf("encode runtime config: %w", err)
	}
	if session.Version == 0 {
		session.Version = 1
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if session.BudgetLimitMicros > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO spending_limits(id,limit_micros) VALUES($1,$2)`, session.ID, session.BudgetLimitMicros); err != nil {
			return fmt.Errorf("create session budget: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
  INSERT INTO live_sessions (
			id, status, playhead_ms, world_state, runtime_config, stream_state,
			version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		session.ID, session.Status, session.Timeline.Playhead.Milliseconds(), world, config, stream,
		session.Version, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create session %s: %w", session.ID, err)
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateSession(ctx context.Context, session *live.LiveSession) error {
	if err := session.Timeline.Validate(); err != nil {
		return fmt.Errorf("validate session timeline: %w", err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin update session: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var currentStatus live.SessionStatus
	var currentVersion int64
	if err := tx.QueryRow(ctx,
		`SELECT status, version FROM live_sessions WHERE id = $1 FOR UPDATE`, session.ID,
	).Scan(&currentStatus, &currentVersion); err != nil {
		return mapNotFound(err, "lock session")
	}
	if session.Version != currentVersion {
		return ErrConflict
	}
	if currentStatus != session.Status {
		current := live.LiveSession{Status: currentStatus}
		if err := current.Transition(session.Status, session.UpdatedAt); err != nil {
			return err
		}
	}
	world, err := json.Marshal(session.World)
	if err != nil {
		return fmt.Errorf("encode world state: %w", err)
	}
	stream, err := json.Marshal(session.Stream)
	if err != nil {
		return fmt.Errorf("encode stream state: %w", err)
	}
	config, err := json.Marshal(runtimeConfig{BudgetLimitMicros: session.BudgetLimitMicros, Models: session.Models, Profile: session.Profile, CommitHorizonMS: session.Timeline.CommitHorizon.Milliseconds()})
	if err != nil {
		return fmt.Errorf("encode runtime config: %w", err)
	}
	tag, err := tx.Exec(ctx, `
		UPDATE live_sessions
		SET status = $2, playhead_ms = $3, world_state = $4, runtime_config = $5,
			stream_state = $6, version = version + 1, updated_at = $7
		WHERE id = $1 AND version = $8`,
		session.ID, session.Status, session.Timeline.Playhead.Milliseconds(), world, config,
		stream, session.UpdatedAt, currentVersion,
	)
	if err != nil {
		return fmt.Errorf("update session %s: %w", session.ID, err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	if err := savePlayedSegments(ctx, tx, session); err != nil {
		return err
	}
	if err := saveCharacterMemory(ctx, tx, session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit session update: %w", err)
	}
	session.Version++
	return nil
}

func (s *Store) ActiveSession(ctx context.Context) (*live.LiveSession, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin active session read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		SELECT id::text, status, playhead_ms, world_state, runtime_config, stream_state,
			version, created_at, updated_at
		FROM live_sessions
		WHERE status NOT IN ('STOPPED', 'FAILED')
		ORDER BY updated_at DESC
		LIMIT 1
		FOR SHARE`)
	session, err := scanSession(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, mapNotFound(err, "read active session")
	}
	segments, err := listSegments(ctx, tx, session.ID)
	if err != nil {
		return nil, err
	}
	session.Timeline.Segments = segments
	if err := session.Timeline.Validate(); err != nil {
		return nil, fmt.Errorf("validate stored timeline: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit active session read: %w", err)
	}
	return session, nil
}

func (s *Store) StartStreamRun(ctx context.Context, sessionID live.SessionID) (string, error) {
	id, err := newUUID()
	if err != nil {
		return "", err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin stream run: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stream_runs (id, session_id, status, started_at)
		VALUES ($1, $2, 'RUNNING', now())`, id, sessionID); err != nil {
		return "", fmt.Errorf("insert stream run: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit stream run: %w", err)
	}
	return id, nil
}

func (s *Store) FinishStreamRun(
	ctx context.Context,
	sessionID live.SessionID,
	id string,
	status string,
	stats streaming.Stats,
	lastError string,
) error {
	if id == "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin finish stream run: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE stream_runs
		SET status = $3, ended_at = now(), bitrate_kbps = $4,
			dropped_frames = $5, gap_total = $6, last_error = $7
		WHERE id = $1 AND session_id = $2 AND ended_at IS NULL`,
		id, sessionID, status, stats.BitrateKbps, stats.DroppedFrames, 0, nullString(lastError),
	)
	if err != nil {
		return fmt.Errorf("finish stream run: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stream run finish: %w", err)
	}
	return nil
}

func scanSession(row scanner) (*live.LiveSession, error) {
	var session live.LiveSession
	var playheadMS int64
	var world, config, stream []byte
	if err := row.Scan(
		&session.ID, &session.Status, &playheadMS, &world, &config, &stream,
		&session.Version, &session.CreatedAt, &session.UpdatedAt,
	); err != nil {
		return nil, err
	}
	var runtime runtimeConfig
	if err := json.Unmarshal(world, &session.World); err != nil {
		return nil, fmt.Errorf("decode world state: %w", err)
	}
	if err := json.Unmarshal(config, &runtime); err != nil {
		return nil, fmt.Errorf("decode runtime config: %w", err)
	}
	if err := json.Unmarshal(stream, &session.Stream); err != nil {
		return nil, fmt.Errorf("decode stream state: %w", err)
	}
	session.Models = runtime.Models
	session.BudgetLimitMicros = runtime.BudgetLimitMicros
	session.Profile = runtime.Profile
	session.Timeline.Playhead = time.Duration(playheadMS) * time.Millisecond
	session.Timeline.CommitHorizon = time.Duration(runtime.CommitHorizonMS) * time.Millisecond
	return &session, nil
}

func savePlayedSegments(ctx context.Context, tx pgx.Tx, session *live.LiveSession) error {
	for _, segment := range session.Timeline.Segments {
		if segment.Status == live.SegmentCommitted || segment.Status == live.SegmentPlaying || segment.Status == live.SegmentPlayed {
			if err := upsertSegment(ctx, tx, session.ID, segment); err != nil {
				return err
			}
		}
	}
	return nil
}
