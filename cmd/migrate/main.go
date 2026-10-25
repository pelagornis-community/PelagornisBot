package main

import (
	"context"
	"log"
	"os"

	"github.com/pelagornis/communityos/internal/storage"
	"github.com/pelagornis/communityos/internal/storage/postgres"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := postgres.Open(context.Background(), url)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()
	if err := storage.Migrate(context.Background(), db.Pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
}
