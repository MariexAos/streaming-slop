package store

import (
	"context"
	"streaming-agent/internal/generation"
	"streaming-agent/internal/live"
)

func (s *Store) UnresolvedAttempts(ctx context.Context) ([]generation.Pending, error) {
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT session_id::text FROM generation_attempts WHERE status NOT IN ('SUCCEEDED','FAILED','CANCELLED')`)
	if err != nil {
		return nil, err
	}
	var ids []live.SessionID
	for rows.Next() {
		var id live.SessionID
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []generation.Pending{}
	for _, id := range ids {
		attempts, err := s.ListPendingAttempts(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, a := range attempts {
			result = append(result, generation.Pending{SessionID: id, Attempt: a})
		}
	}
	return result, nil
}
