package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/pelagornis/pelagornis-bot/internal/bot"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/store"
)

func main() {
	_ = godotenv.Load()

	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	st, err := store.New(dataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	b, err := bot.New(cfg, st)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}
	if err := b.Start(); err != nil {
		log.Fatalf("start: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("bot: shutting down")
	b.Stop()
}
