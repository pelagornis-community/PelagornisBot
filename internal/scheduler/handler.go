package scheduler

import (
	"context"
	"fmt"
	"github.com/pelagornis/communityos/internal/modules/feeds"
	"github.com/pelagornis/communityos/internal/storage/postgres"
	"github.com/pelagornis/communityos/internal/workflow"
)

type AppHandler struct {
	Feeds    feeds.Poller
	Workflow workflow.Executor
	Delivery func(context.Context, postgres.Job) error
}

func (h AppHandler) Handle(c context.Context, j postgres.Job) error {
	switch j.Kind {
	case "feed.poll":
		return h.Feeds.Handle(c, j)
	case "workflow.action":
		return h.Workflow.Handle(c, j)
	case "feed.delivery":
		return h.Delivery(c, j)
	default:
		return fmt.Errorf("unknown job kind: %s", j.Kind)
	}
}
