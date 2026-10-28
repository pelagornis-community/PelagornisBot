package scheduler

import (
	"context"
	"fmt"
	"github.com/pelagornis/pelagornis-bot/internal/modules/feeds"
	githubapi "github.com/pelagornis/pelagornis-bot/internal/modules/github"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"github.com/pelagornis/pelagornis-bot/internal/workflow"
)

type AppHandler struct {
	Feeds    feeds.Poller
	Workflow workflow.Executor
	Delivery func(context.Context, sqlite.Job) error
	GitHub   githubapi.WebhookService
}

func (h AppHandler) Handle(c context.Context, j sqlite.Job) error {
	switch j.Kind {
	case "feed.poll":
		return h.Feeds.Handle(c, j)
	case "workflow.action":
		return h.Workflow.Handle(c, j)
	case "feed.delivery":
		return h.Delivery(c, j)
	case "github.webhook":
		return h.GitHub.Handle(c, j)
	default:
		return fmt.Errorf("unknown job kind: %s", j.Kind)
	}
}
