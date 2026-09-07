package store

import (
	"context"
	"encoding/json"
	"fmt"
	"streaming-agent/internal/live"
)

func (s *Store) PublishCharacter(ctx context.Context, p live.CharacterProfile) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `INSERT INTO characters(id) VALUES($1) ON CONFLICT DO NOTHING`, p.CharacterID); err != nil {
		return err
	}
	for _, ref := range []live.ReferenceAsset{p.FirstFrame, p.LastFrame} {
		if _, err = tx.Exec(ctx, `INSERT INTO reference_assets(id,path,mime,width,height) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, ref.ID, ref.Path, ref.MIME, ref.Width, ref.Height); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_versions(id,character_id,profile) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, p.ID, p.CharacterID, data); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET current_version=$2 WHERE id=$1`, p.CharacterID, p.ID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_selection VALUES(true,$1) ON CONFLICT DO NOTHING`, p.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Character(ctx context.Context, id string) (live.CharacterProfile, error) {
	var data []byte
	var p live.CharacterProfile
	err := s.pool.QueryRow(ctx, `SELECT profile FROM character_versions WHERE id=COALESCE(NULLIF($1,''),(SELECT version_id FROM character_selection WHERE singleton))`, id).Scan(&data)
	if err != nil {
		return p, mapNotFound(err, "read character")
	}
	return p, json.Unmarshal(data, &p)
}
func (s *Store) Characters(ctx context.Context) ([]live.CharacterProfile, error) {
	rows, err := s.pool.Query(ctx, `SELECT profile FROM character_versions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []live.CharacterProfile{}
	for rows.Next() {
		var data []byte
		var p live.CharacterProfile
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}
func (s *Store) SelectCharacter(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `UPDATE character_selection SET version_id=$1 WHERE singleton AND EXISTS(SELECT 1 FROM character_versions WHERE id=$1)`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("select character: %w", ErrNotFound)
	}
	return nil
}
