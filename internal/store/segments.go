package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"streaming-agent/internal/live"
)

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
	if err := validateSegmentUpdate(existing, segment, direction); err != nil {
		return err
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

func selectSegment(ctx context.Context, tx pgx.Tx, sessionID live.SessionID, id live.SegmentID, lock bool) (live.Segment, error) {
	query := `
		SELECT s.id::text, s.sequence, s.start_ms, s.end_ms, s.direction, s.status, s.version, s.plan_revision,
			a.id::text, a.attempt_id::text, a.source, a.uri, a.normalized_uri,
			a.duration_ms, a.width, a.height, a.fps, a.checksum, a.verified_at, a.observed
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
		SELECT s.id::text, s.sequence, s.start_ms, s.end_ms, s.direction, s.status, s.version, s.plan_revision,
			a.id::text, a.attempt_id::text, a.source, a.uri, a.normalized_uri,
			a.duration_ms, a.width, a.height, a.fps, a.checksum, a.verified_at, a.observed
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

func scanSegment(row scanner) (live.Segment, error) {
	var segment live.Segment
	var startMS, endMS int64
	var direction, observed []byte
	var assetID, attemptID, source, uri, normalizedURI, checksum sql.NullString
	var durationMS, width, height, fps sql.NullInt64
	var verifiedAt sql.NullTime
	if err := row.Scan(
		&segment.ID, &segment.Sequence, &startMS, &endMS, &direction, &segment.Status, &segment.Version, &segment.PlanRevision,
		&assetID, &attemptID, &source, &uri, &normalizedURI, &durationMS, &width, &height, &fps, &checksum, &verifiedAt, &observed,
	); err != nil {
		return live.Segment{}, err
	}
	if err := json.Unmarshal(direction, &segment.Direction); err != nil {
		return live.Segment{}, fmt.Errorf("decode segment direction: %w", err)
	}
	segment.Start = time.Duration(startMS) * time.Millisecond
	segment.End = time.Duration(endMS) * time.Millisecond
	if assetID.Valid {
		var delta live.WorldDelta
		if err := json.Unmarshal(observed, &delta); err != nil {
			return live.Segment{}, err
		}
		segment.Asset = &live.VideoAsset{
			Observed: delta, ID: live.AssetID(assetID.String), AttemptID: live.AttemptID(attemptID.String),
			Source: live.AssetSource(source.String), URI: uri.String, NormalizedURI: normalizedURI.String,
			Duration: time.Duration(durationMS.Int64) * time.Millisecond,
			Width:    int(width.Int64), Height: int(height.Int64), FPS: int(fps.Int64),
			Checksum: checksum.String, VerifiedAt: verifiedAt.Time,
		}
	}
	return segment, nil
}

// A runtime snapshot may contain several completed in-memory transitions.
func advanceSegment(segment *live.Segment, target live.SegmentStatus) error {
	order := []live.SegmentStatus{live.SegmentPlanned, live.SegmentGenerating, live.SegmentReady, live.SegmentCommitted, live.SegmentPlaying, live.SegmentPlayed}
	for i, status := range order {
		if status != segment.Status {
			continue
		}
		for _, next := range order[i+1:] {
			if err := segment.Transition(next); err != nil {
				return err
			}
			if next == target {
				return nil
			}
		}
		break
	}
	return fmt.Errorf("cannot advance segment to %s", target)
}

func validateSegmentUpdate(existing, segment live.Segment, direction []byte) error {
	if existing.PlanRevision != segment.PlanRevision {
		return ErrConflict
	}
	if existing.Sequence != segment.Sequence || existing.Start != segment.Start || existing.End != segment.End {
		return errors.New("segment identity and time range cannot change")
	}
	if existing.Status != segment.Status {
		candidate := existing
		candidate.Asset = segment.Asset
		if err := advanceSegment(&candidate, segment.Status); err != nil {
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

	return nil
}
