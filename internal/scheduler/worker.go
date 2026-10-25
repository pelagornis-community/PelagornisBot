package scheduler

import (
	"context"
	"fmt"
	"github.com/pelagornis/communityos/internal/storage/postgres"
	"log/slog"
	"time"
)

type Handler interface {
	Handle(context.Context, postgres.Job) error
}
type Worker struct {
	Store       *postgres.Store
	Handler     Handler
	Concurrency int
}

func (w Worker) Run(ctx context.Context) {
	if w.Concurrency < 1 {
		w.Concurrency = 1
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	sem := make(chan struct{}, w.Concurrency)
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			jobs, e := w.Store.ClaimJobs(ctx, w.Concurrency)
			if e != nil {
				slog.Warn("claim jobs", "error", e)
				continue
			}
			for _, j := range jobs {
				sem <- struct{}{}
				go func(j postgres.Job) {
					defer func() { <-sem }()
					e := w.Handler.Handle(ctx, j)
					if x := w.Store.FinishJob(ctx, j.ID, e); x != nil {
						slog.Error("finish job", "error", x)
					}
					if e != nil {
						slog.Warn("job failed", "id", j.ID, "kind", j.Kind, "error", e)
					}
				}(j)
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

func (RejectUnknown) Handle(_ context.Context, j postgres.Job) error {
	return fmt.Errorf("unknown job kind: %s", j.Kind)
}
