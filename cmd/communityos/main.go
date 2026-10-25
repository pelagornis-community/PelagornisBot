package main

import (
	"context"
	"errors"
	"github.com/pelagornis/communityos/internal/config"
	"github.com/pelagornis/communityos/internal/discord/delivery"
	"github.com/pelagornis/communityos/internal/discord/gateway"
	"github.com/pelagornis/communityos/internal/httpserver"
	"github.com/pelagornis/communityos/internal/modules/feeds"
	"github.com/pelagornis/communityos/internal/scheduler"
	"github.com/pelagornis/communityos/internal/storage"
	"github.com/pelagornis/communityos/internal/storage/postgres"
	"github.com/pelagornis/communityos/internal/workflow"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func main() {
	runtime.GOMAXPROCS(2)
	cfg, e := config.Load()
	if e != nil {
		slog.Error("configuration", "error", e)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, e := postgres.Open(ctx, cfg.DatabaseURL)
	if e != nil {
		slog.Error("database", "error", e)
		os.Exit(1)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db.Pool); e != nil {
		slog.Error("migrations", "error", e)
		os.Exit(1)
	}
	gw, e := gateway.New(cfg, db)
	if e != nil {
		slog.Error("discord", "error", e)
		os.Exit(1)
	}
	if e = gw.Open(); e != nil {
		slog.Error("discord connect", "error", e)
		os.Exit(1)
	}
	defer gw.Close()
	discordDelivery := delivery.Service{Discord: gw.Session()}
	poller := feeds.Poller{Store: db, Client: feeds.New()}
	handler := scheduler.AppHandler{Feeds: poller, Workflow: workflow.Executor{Delivery: discordDelivery}, Delivery: func(c context.Context, j postgres.Job) error { return discordDelivery.HandleFeed(c, db, j) }}
	go scheduler.Worker{Store: db, Handler: handler, Concurrency: cfg.Workers}.Run(ctx)
	go scheduler.RunDueEnqueuer(ctx, poller)
	srv := httpserver.New(cfg, db)
	go srv.Run(ctx)
	<-ctx.Done()
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := srv.Close(c); e != nil && !errors.Is(e, http.ErrServerClosed) {
		slog.Error("shutdown", "error", e)
	}
}
