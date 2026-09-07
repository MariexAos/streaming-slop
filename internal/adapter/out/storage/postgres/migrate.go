package postgres

import (
	"context"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func (s *Store) Migrate(ctx context.Context) error {
	db := stdlib.OpenDB(*s.pool.Config().ConnConfig)
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("apply postgres migrations: %w", err)
	}
	return nil
}
