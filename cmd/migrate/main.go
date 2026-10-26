package main

import (
	"context"
	"log"
	"os"

	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

func main() {
	path := os.Getenv("SQLITE_PATH")
	if path == "" {
		path = "data/pelagornisbot.db"
	}
	db, err := sqlite.Open(context.Background(), path)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(context.Background(), db.DB); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}
