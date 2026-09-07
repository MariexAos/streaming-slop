package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
	"streaming-agent/internal/streaming"
)

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return New(pool), nil
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Close() {
	s.pool.Close()
}

type runtimeConfig struct {
	CommitHorizonMS int64 `json:"commitHorizonMs"`
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
	config, err := json.Marshal(runtimeConfig{CommitHorizonMS: session.Timeline.CommitHorizon.Milliseconds()})
	if err != nil {
		return fmt.Errorf("encode runtime config: %w", err)
	}
	if session.Version == 0 {
		session.Version = 1
	}
	_, err = s.pool.Exec(ctx, `
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
	return nil
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
	config, err := json.Marshal(runtimeConfig{CommitHorizonMS: session.Timeline.CommitHorizon.Milliseconds()})
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

func (s *Store) ListAttempts(ctx context.Context, sessionID live.SessionID) ([]generation.Attempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, segment_id::text, attempt_no, provider, idempotency_key,
			provider_job_id, status, error_code, error_message, latency_ms, cost_cny,
			version, created_at, updated_at, submitted_at, started_at, finished_at
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

func (s *Store) UpsertSegment(ctx context.Context, sessionID live.SessionID, segment live.Segment) error {
	return s.UpsertSegments(ctx, sessionID, []live.Segment{segment})
}

func (s *Store) UpsertSegments(ctx context.Context, sessionID live.SessionID, segments []live.Segment) error {
	if len(segments) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin segment upsert: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	for i := range segments {
		if err := upsertSegment(ctx, tx, sessionID, segments[i]); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit segment upsert: %w", err)
	}
	return nil
}

func upsertSegment(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, segment live.Segment) error {
	if segment.Status == live.SegmentReady && segment.Asset != nil {
		return errors.New("use MarkReady to persist a ready asset")
	}
	if err := segment.Validate(); err != nil {
		return fmt.Errorf("validate segment %s: %w", segment.ID, err)
	}
	direction, err := json.Marshal(segment.Direction)
	if err != nil {
		return fmt.Errorf("encode segment direction: %w", err)
	}

	existing, err := selectSegment(ctx, tx, sessionID, segment.ID, true)
	if errors.Is(err, ErrNotFound) {
		_, err = tx.Exec(ctx, `
			INSERT INTO segments (
				id, session_id, sequence, start_ms, end_ms, direction, status, version
			) VALUES ($1, $2, $3, $4, $5, $6, $7, 1)`,
			segment.ID, sessionID, segment.Sequence, segment.Start.Milliseconds(), segment.End.Milliseconds(),
			direction, segment.Status,
		)
		if err != nil {
			return fmt.Errorf("insert segment %s: %w", segment.ID, err)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if existing.Sequence != segment.Sequence || existing.Start != segment.Start || existing.End != segment.End {
		return errors.New("segment identity and time range cannot change")
	}
	if existing.Status != segment.Status {
		candidate := existing
		candidate.Asset = segment.Asset
		if err := candidate.Transition(segment.Status); err != nil {
			return err
		}
	}
	if existing.Status == live.SegmentCommitted || existing.Status == live.SegmentPlaying || existing.Status == live.SegmentPlayed {
		existingDirection, err := json.Marshal(existing.Direction)
		if err != nil {
			return fmt.Errorf("encode stored segment direction: %w", err)
		}
		if !bytes.Equal(existingDirection, direction) {
			return errors.New("committed segment direction cannot change")
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE segments
		SET direction = $3, status = $4, version = version + 1, updated_at = now()
		WHERE session_id = $1 AND id = $2`,
		sessionID, segment.ID, direction, segment.Status,
	)
	if err != nil {
		return fmt.Errorf("update segment %s: %w", segment.ID, err)
	}
	return nil
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
	if segment.Status != live.SegmentPlanned && segment.Status != live.SegmentGenerating {
		return fmt.Errorf("segment %s in status %s cannot start generation", segment.ID, segment.Status)
	}
	var active bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM generation_attempts
			WHERE segment_id = $1 AND status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED')
		)`, attempt.SegmentID).Scan(&active); err != nil {
		return fmt.Errorf("check active attempt: %w", err)
	}
	if active {
		return ErrActiveAttempt
	}
	var count int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FROM generation_attempts WHERE segment_id = $1`, attempt.SegmentID,
	).Scan(&count); err != nil {
		return fmt.Errorf("count segment attempts: %w", err)
	}
	if count >= generation.MaxAttemptsPerSegment || attempt.Number != count+1 {
		return errors.New("attempt number does not follow the segment attempt history")
	}
	if attempt.Version == 0 {
		attempt.Version = 1
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO generation_attempts (
			id, session_id, segment_id, attempt_no, provider, idempotency_key, status,
			version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		attempt.ID, sessionID, attempt.SegmentID, attempt.Number, attempt.Provider,
		attempt.IdempotencyKey, attempt.Status, attempt.Version, attempt.CreatedAt, attempt.UpdatedAt,
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
			version, created_at, updated_at, submitted_at, started_at, finished_at
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

func (s *Store) MarkReady(
	ctx context.Context,
	sessionID live.SessionID,
	attemptID live.AttemptID,
	asset live.VideoAsset,
) error {
	if asset.ID == "" {
		id, err := newUUID()
		if err != nil {
			return err
		}
		asset.ID = live.AssetID(id)
	}
	if !asset.Playable() {
		return errors.New("asset is not playable")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark ready: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	var segmentID live.SegmentID
	if err := tx.QueryRow(ctx,
		`SELECT segment_id::text FROM generation_attempts WHERE session_id = $1 AND id = $2`, sessionID, attemptID,
	).Scan(&segmentID); err != nil {
		return mapNotFound(err, "find attempt segment")
	}
	segment, err := selectSegment(ctx, tx, sessionID, segmentID, true)
	if err != nil {
		return err
	}
	attempt, err := selectAttempt(ctx, tx, sessionID, attemptID, true)
	if err != nil {
		return err
	}
	if attempt.Status != generation.AttemptPreparing {
		return fmt.Errorf("attempt %s in status %s cannot complete", attempt.ID, attempt.Status)
	}
	if asset.AttemptID != "" && asset.AttemptID != attemptID {
		return errors.New("asset attempt id does not match")
	}
	asset.AttemptID = attemptID
	segment.Asset = &asset
	if err := segment.Transition(live.SegmentReady); err != nil {
		return err
	}
	if err := attempt.Transition(generation.AttemptSucceeded, asset.VerifiedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE assets SET is_current = false WHERE segment_id = $1 AND is_current`, segmentID,
	); err != nil {
		return fmt.Errorf("retire current asset: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO assets (
			id, session_id, segment_id, attempt_id, source, uri, normalized_uri,
			duration_ms, width, height, fps, checksum, verified_at, is_current
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, true)`,
		asset.ID, sessionID, segmentID, attemptID, asset.Source, asset.URI, asset.NormalizedURI,
		asset.Duration.Milliseconds(), asset.Width, asset.Height, asset.FPS, nullString(asset.Checksum), asset.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("insert ready asset: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE generation_attempts
		SET status = 'SUCCEEDED', latency_ms = $3, finished_at = $4,
			updated_at = $4, version = version + 1
		WHERE session_id = $1 AND id = $2`,
		sessionID, attemptID, nullDurationMS(attempt.Latency), asset.VerifiedAt,
	); err != nil {
		return fmt.Errorf("complete attempt: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE segments
		SET status = 'READY', current_asset_id = $3, version = version + 1, updated_at = $4
		WHERE session_id = $1 AND id = $2`,
		sessionID, segmentID, asset.ID, asset.VerifiedAt,
	); err != nil {
		return fmt.Errorf("mark segment ready: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mark ready: %w", err)
	}
	return nil
}

func (s *Store) MarkFallbackReady(
	ctx context.Context,
	sessionID live.SessionID,
	segmentID live.SegmentID,
	asset live.VideoAsset,
) error {
	if asset.Source != live.AssetSourceFallback || !asset.Playable() {
		return errors.New("fallback asset is not playable")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mark fallback ready: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockSession(ctx, tx, sessionID); err != nil {
		return err
	}
	segment, err := selectSegment(ctx, tx, sessionID, segmentID, true)
	if err != nil {
		return err
	}
	if segment.Status == live.SegmentReady && segment.Asset != nil && segment.Asset.ID == asset.ID {
		return tx.Commit(ctx)
	}
	if segment.Status == live.SegmentPlanned {
		if err := segment.Transition(live.SegmentGenerating); err != nil {
			return err
		}
	}
	if segment.Status != live.SegmentGenerating {
		return fmt.Errorf("segment %s in status %s cannot use fallback", segment.ID, segment.Status)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE assets SET is_current = false WHERE segment_id = $1 AND is_current`, segmentID,
	); err != nil {
		return fmt.Errorf("retire current asset: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO assets (
			id, session_id, segment_id, attempt_id, source, uri, normalized_uri,
			duration_ms, width, height, fps, checksum, verified_at, is_current
		) VALUES ($1, $2, $3, NULL, $4, $5, $6, $7, $8, $9, $10, $11, $12, true)`,
		asset.ID, sessionID, segmentID, asset.Source, asset.URI, asset.NormalizedURI,
		asset.Duration.Milliseconds(), asset.Width, asset.Height, asset.FPS,
		nullString(asset.Checksum), asset.VerifiedAt,
	); err != nil {
		return fmt.Errorf("insert fallback asset: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE segments
		SET status = 'READY', current_asset_id = $3, version = version + 1, updated_at = $4
		WHERE session_id = $1 AND id = $2`,
		sessionID, segmentID, asset.ID, asset.VerifiedAt,
	); err != nil {
		return fmt.Errorf("mark fallback segment ready: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fallback ready: %w", err)
	}
	return nil
}

func lockSession(ctx context.Context, tx pgx.Tx, id live.SessionID) error {
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT true FROM live_sessions WHERE id = $1 FOR UPDATE`, id,
	).Scan(&exists); err != nil {
		return mapNotFound(err, "lock session")
	}
	return nil
}

func selectSegment(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, id live.SegmentID, lock bool) (live.Segment, error) {
	query := `
		SELECT s.id::text, s.sequence, s.start_ms, s.end_ms, s.direction, s.status, s.version,
			a.id::text, a.attempt_id::text, a.source, a.uri, a.normalized_uri,
			a.duration_ms, a.width, a.height, a.fps, a.checksum, a.verified_at
		FROM segments s
		LEFT JOIN assets a ON a.id = s.current_asset_id
		WHERE s.session_id = $1 AND s.id = $2`
	if lock {
		query += ` FOR UPDATE OF s`
	}
	segment, err := scanSegment(tx.QueryRow(ctx, query, sessionID, id))
	if err != nil {
		return live.Segment{}, mapNotFound(err, "read segment")
	}
	return segment, nil
}

func listSegments(ctx context.Context, tx pgx.Tx, sessionID live.SessionID) ([]live.Segment, error) {
	rows, err := tx.Query(ctx, `
		SELECT s.id::text, s.sequence, s.start_ms, s.end_ms, s.direction, s.status, s.version,
			a.id::text, a.attempt_id::text, a.source, a.uri, a.normalized_uri,
			a.duration_ms, a.width, a.height, a.fps, a.checksum, a.verified_at
		FROM segments s
		LEFT JOIN assets a ON a.id = s.current_asset_id
		WHERE s.session_id = $1
		ORDER BY s.sequence`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list segments: %w", err)
	}
	defer rows.Close()
	var segments []live.Segment
	for rows.Next() {
		segment, err := scanSegment(rows)
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate segments: %w", err)
	}
	return segments, nil
}

type scanner interface {
	Scan(...any) error
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
	session.Timeline.Playhead = time.Duration(playheadMS) * time.Millisecond
	session.Timeline.CommitHorizon = time.Duration(runtime.CommitHorizonMS) * time.Millisecond
	return &session, nil
}

func scanSegment(row scanner) (live.Segment, error) {
	var segment live.Segment
	var startMS, endMS int64
	var direction []byte
	var assetID, attemptID, source, uri, normalizedURI, checksum sql.NullString
	var durationMS, width, height, fps sql.NullInt64
	var verifiedAt sql.NullTime
	if err := row.Scan(
		&segment.ID, &segment.Sequence, &startMS, &endMS, &direction, &segment.Status, &segment.Version,
		&assetID, &attemptID, &source, &uri, &normalizedURI, &durationMS, &width, &height, &fps, &checksum, &verifiedAt,
	); err != nil {
		return live.Segment{}, err
	}
	if err := json.Unmarshal(direction, &segment.Direction); err != nil {
		return live.Segment{}, fmt.Errorf("decode segment direction: %w", err)
	}
	segment.Start = time.Duration(startMS) * time.Millisecond
	segment.End = time.Duration(endMS) * time.Millisecond
	if assetID.Valid {
		segment.Asset = &live.VideoAsset{
			ID: live.AssetID(assetID.String), AttemptID: live.AttemptID(attemptID.String),
			Source: live.AssetSource(source.String), URI: uri.String, NormalizedURI: normalizedURI.String,
			Duration: time.Duration(durationMS.Int64) * time.Millisecond,
			Width:    int(width.Int64), Height: int(height.Int64), FPS: int(fps.Int64),
			Checksum: checksum.String, VerifiedAt: verifiedAt.Time,
		}
	}
	return segment, nil
}

func selectAttempt(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, id live.AttemptID, lock bool) (generation.Attempt, error) {
	query := `
		SELECT id::text, segment_id::text, attempt_no, provider, idempotency_key,
			provider_job_id, status, error_code, error_message, latency_ms, cost_cny,
			version, created_at, updated_at, submitted_at, started_at, finished_at
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
	var providerJobID, errorCode, errorMessage sql.NullString
	var latencyMS sql.NullInt64
	var cost sql.NullFloat64
	var submittedAt, startedAt, finishedAt sql.NullTime
	if err := row.Scan(
		&attempt.ID, &attempt.SegmentID, &attempt.Number, &attempt.Provider, &attempt.IdempotencyKey,
		&providerJobID, &attempt.Status, &errorCode, &errorMessage, &latencyMS, &cost,
		&attempt.Version, &attempt.CreatedAt, &attempt.UpdatedAt, &submittedAt, &startedAt, &finishedAt,
	); err != nil {
		return generation.Attempt{}, err
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

func mapNotFound(err error, operation string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullDurationMS(value time.Duration) any {
	if value == 0 {
		return nil
	}
	return value.Milliseconds()
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	var encoded [36]byte
	hex.Encode(encoded[0:8], value[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], value[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], value[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], value[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], value[10:16])
	return string(encoded[:]), nil
}
