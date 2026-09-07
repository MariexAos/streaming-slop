package postgres

import (
	"context"
	"fmt"
	"strings"
)

const (
	MiniMaxAPIKey = "minimax_api_key"
	QwenAPIKey    = "qwen_api_key"
)

type ProviderSecrets struct {
	MiniMaxAPIKey string
	QwenAPIKey    string
}

func (s *Store) ProviderSecrets(ctx context.Context) (ProviderSecrets, error) {
	rows, err := s.pool.Query(ctx, `SELECT name, value FROM provider_secrets`)
	if err != nil {
		return ProviderSecrets{}, fmt.Errorf("read provider secrets: %w", err)
	}
	defer rows.Close()

	var secrets ProviderSecrets
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return ProviderSecrets{}, fmt.Errorf("scan provider secret: %w", err)
		}
		switch name {
		case MiniMaxAPIKey:
			secrets.MiniMaxAPIKey = value
		case QwenAPIKey:
			secrets.QwenAPIKey = value
		}
	}
	if err := rows.Err(); err != nil {
		return ProviderSecrets{}, fmt.Errorf("iterate provider secrets: %w", err)
	}
	return secrets, nil
}

func (s *Store) SaveProviderSecret(ctx context.Context, name, value string) error {
	if name != MiniMaxAPIKey && name != QwenAPIKey {
		return fmt.Errorf("unknown provider secret %q", name)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("provider secret is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO provider_secrets (name, value, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		name, value,
	)
	if err != nil {
		return fmt.Errorf("save provider secret: %w", err)
	}
	return nil
}
