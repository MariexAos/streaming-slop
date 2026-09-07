package store

import (
	"context"
	"encoding/json"
	"streaming-agent/internal/live"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CharacterMemory(ctx context.Context, id string) ([]live.WorldEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT facts FROM character_memories WHERE character_id=$1 ORDER BY updated_at DESC LIMIT 3`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []live.WorldEvent
	for rows.Next() {
		var data []byte
		var batch []live.WorldEvent
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &batch); err != nil {
			return nil, err
		}
		events = append(events, batch...)
	}
	if len(events) > 40 {
		events = events[:40]
	}
	return events, rows.Err()
}

func saveCharacterMemory(ctx context.Context, tx pgx.Tx, session *live.LiveSession) error {
	if session.Profile != nil {
		var facts []live.WorldEvent
		for _, event := range session.World.Recent {
			if event.Kind == "visual_observation" {
				facts = append(facts, event)
			}
		}
		data, err := json.Marshal(facts)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO character_memories(character_id,session_id,facts) VALUES($1,$2,$3) ON CONFLICT(character_id,session_id) DO UPDATE SET facts=EXCLUDED.facts,updated_at=now()`, session.Profile.CharacterID, session.ID, data); err != nil {
			return err
		}
	}

	return nil
}
