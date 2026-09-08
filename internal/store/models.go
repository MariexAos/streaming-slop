package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"streaming-agent/internal/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ModelSettings(ctx context.Context) (models.Settings, bool, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, "SELECT settings FROM model_settings WHERE singleton=true").Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Settings{}, false, nil
	}
	if err != nil {
		return models.Settings{}, false, fmt.Errorf("read model settings: %w", err)
	}
	var settings models.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, false, fmt.Errorf("decode model settings: %w", err)
	}
	return settings, true, models.Validate(settings)
}
func (s *Store) SaveModelSettings(ctx context.Context, settings models.Settings) error {
	if err := models.Validate(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO model_settings(singleton,settings) VALUES(true,$1) ON CONFLICT(singleton) DO UPDATE SET settings=excluded.settings,updated_at=now()`, data)
	if err != nil {
		return fmt.Errorf("save model settings: %w", err)
	}
	return nil
}
