package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func TestSQLiteMigrationIsRepeatable(t *testing.T) {
	db, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "pelagornisbot.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = storage.Migrate(context.Background(), db.DB); err != nil {
		t.Fatalf("first migration: %v", err)
	}
	if err = storage.Migrate(context.Background(), db.DB); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var count int
	if err = db.DB.QueryRow(`select count(*) from sqlite_master where type='table' and name in ('forms','jobs','form_review_channels')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected SQLite schema tables, got %d", count)
	}
}
