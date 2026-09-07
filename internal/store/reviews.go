package store

import (
	"context"
	"encoding/json"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func (s *Store) SaveReview(ctx context.Context, id live.AttemptID, review generation.Review) error {
	data, err := json.Marshal(review)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO visual_reviews(attempt_id,review) VALUES($1,$2) ON CONFLICT(attempt_id) DO NOTHING`, id, data)
	return err
}
