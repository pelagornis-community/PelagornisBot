package storage

import (
	"context"
	"database/sql"
	"embed"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if e := goose.SetDialect("sqlite3"); e != nil {
		return e
	}
	return goose.UpContext(ctx, db, "migrations")
}
