package scheduler

import (
	"context"
	"fmt"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"log/slog"
	"time"
)

type Handler interface {
	Handle(context.Context, sqlite.Job) error
}
type Worker struct {
	Store   *sqlite.Store
	Handler Handler
}

func (w Worker) Run(ctx context.Context) {
	slog.Info("scheduler worker started")
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			jobs, e := w.Store.ClaimJobs(ctx, 1)
			if e != nil {
				slog.Warn("claim jobs", "error", e)
				continue
			}
			if len(jobs) > 0 {
				slog.Info("scheduler claimed jobs", "count", len(jobs))
			}
			for _, j := range jobs {
				slog.Info("scheduler handling job", "id", j.ID, "kind", j.Kind, "attempt", j.Attempts)
				e := w.Handler.Handle(ctx, j)
				if x := w.Store.FinishJob(ctx, j.ID, e); x != nil {
					slog.Error("finish job", "error", x)
				}
				if e != nil {
					slog.Warn("job failed", "id", j.ID, "kind", j.Kind, "error", e)
				} else {
					slog.Info("scheduler completed job", "id", j.ID, "kind", j.Kind)
				}
			}
		}
	}
}

type DueEnqueuer interface{ EnqueueDue(context.Context) error }

func RunDueEnqueuer(ctx context.Context, e DueEnqueuer) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		if err := e.EnqueueDue(ctx); err != nil {
			slog.Warn("enqueue due work", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

type RejectUnknown struct{}

func (RejectUnknown) Handle(_ context.Context, j sqlite.Job) error {
	return fmt.Errorf("unknown job kind: %s", j.Kind)
}
