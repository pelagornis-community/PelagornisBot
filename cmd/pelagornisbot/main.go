package main

import (
	"context"
	"errors"
	"github.com/pelagornis/pelagornis-bot/internal/config"
	"github.com/pelagornis/pelagornis-bot/internal/discord/delivery"
	"github.com/pelagornis/pelagornis-bot/internal/discord/gateway"
	"github.com/pelagornis/pelagornis-bot/internal/httpserver"
	"github.com/pelagornis/pelagornis-bot/internal/modules/feeds"
	"github.com/pelagornis/pelagornis-bot/internal/scheduler"
	"github.com/pelagornis/pelagornis-bot/internal/storage"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"github.com/pelagornis/pelagornis-bot/internal/workflow"
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
	db, e := sqlite.Open(ctx, cfg.SQLitePath)
	if e != nil {
		slog.Error("database", "error", e)
		os.Exit(1)
	}
	defer db.Close()
	if e = storage.Migrate(ctx, db.DB); e != nil {
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
	if e = poller.EnqueueAll(ctx); e != nil {
		slog.Warn("enqueue startup feed catch-up", "error", e)
	}
	handler := scheduler.AppHandler{Feeds: poller, Workflow: workflow.Executor{Delivery: discordDelivery}, Delivery: func(c context.Context, j sqlite.Job) error { return discordDelivery.HandleFeed(c, db, j) }}
	go scheduler.Worker{Store: db, Handler: handler}.Run(ctx)
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
