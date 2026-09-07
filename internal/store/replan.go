package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"streaming-agent/internal/live"
)

// ReplanSegments replaces one future suffix atomically. Attempts stay queryable
// so running, superseded work can still be reconciled and accounted for.
func (s *Store) ReplanSegments(ctx context.Context, id live.SessionID, segments []live.Segment) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin replan: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var playhead int64
	var config []byte
	if err := tx.QueryRow(ctx, `SELECT playhead_ms, runtime_config FROM live_sessions WHERE id=$1 FOR UPDATE`, id).Scan(&playhead, &config); err != nil {
		return err
	}
	var runtime runtimeConfig
	if err := json.Unmarshal(config, &runtime); err != nil {
		return err
	}
	for _, next := range segments {
		current, err := selectSegment(ctx, tx, id, next.ID, true)
		if err != nil {
			return err
		}
		if current.PlanRevision+1 != next.PlanRevision {
			return ErrConflict
		}
		if err := current.Replan(next.Direction, time.Duration(playhead+runtime.CommitHorizonMS)*time.Millisecond); err != nil {
			return err
		}
		direction, err := json.Marshal(current.Direction)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE assets SET is_current=false WHERE segment_id=$1 AND is_current`, current.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE segments SET direction=$3, status='PLANNED', current_asset_id=NULL, plan_revision=$4, version=version+1, updated_at=now() WHERE session_id=$1 AND id=$2`, id, current.ID, direction, current.PlanRevision); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
