package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) MarkReady(
	ctx context.Context,
	sessionID live.SessionID,
	attemptID live.AttemptID,
	asset live.VideoAsset,
) error {
	if asset.ID == "" {
		asset.ID = live.AssetID(uuid.NewString())
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
	segment, attempt, err := lockCompletion(ctx, tx, sessionID, attemptID)
	if err != nil {
		return err
	}
	if attempt.PlanRevision != segment.PlanRevision {
		return ErrConflict
	}
	if attempt.Status == generation.AttemptSucceeded && segment.Asset != nil && segment.Asset.AttemptID == attemptID && segment.Asset.ID == asset.ID {
		return tx.Commit(ctx)
	}
	segmentID := segment.ID
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
	if err := insertReadyAsset(ctx, tx, sessionID, segmentID, attemptID, asset); err != nil {
		return err
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

func lockCompletion(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, attemptID live.AttemptID) (live.Segment, generation.Attempt, error) {
	var segmentID live.SegmentID
	if err := tx.QueryRow(ctx,
		`SELECT segment_id::text FROM generation_attempts WHERE session_id = $1 AND id = $2`, sessionID, attemptID,
	).Scan(&segmentID); err != nil {
		return live.Segment{}, generation.Attempt{}, mapNotFound(err, "find attempt segment")
	}
	segment, err := selectSegment(ctx, tx, sessionID, segmentID, true)
	if err != nil {
		return live.Segment{}, generation.Attempt{}, err
	}
	attempt, err := selectAttempt(ctx, tx, sessionID, attemptID, true)
	if err != nil {
		return live.Segment{}, generation.Attempt{}, err
	}
	return segment, attempt, nil
}

func insertReadyAsset(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, segmentID live.SegmentID, attemptID live.AttemptID, asset live.VideoAsset) error {
	if _, err := tx.Exec(ctx,
		`UPDATE assets SET is_current = false WHERE segment_id = $1 AND is_current`, segmentID,
	); err != nil {
		return fmt.Errorf("retire current asset: %w", err)
	}
	observed, err := json.Marshal(asset.Observed)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO assets (
			id, session_id, segment_id, attempt_id, source, uri, normalized_uri,
			duration_ms, width, height, fps, checksum, verified_at, is_current, observed
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, true, $14)`,
		asset.ID, sessionID, segmentID, attemptID, asset.Source, asset.URI, asset.NormalizedURI,
		asset.Duration.Milliseconds(), asset.Width, asset.Height, asset.FPS, nullString(asset.Checksum), asset.VerifiedAt, observed,
	)
	if err != nil {
		return fmt.Errorf("insert ready asset: %w", err)
	}

	return nil
}
