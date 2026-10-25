package storage

import (
	"context"
	"embed"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, p *pgxpool.Pool) error {
	goose.SetBaseFS(migrations)
	db := stdlib.OpenDB(*p.Config().ConnConfig)
	defer db.Close()
	return goose.UpContext(ctx, db, "migrations")
}
