package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pelagornis/pelagornis-bot/internal/discord/delivery"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

type Executor struct{ Delivery delivery.Service }
type actionJob struct {
	Action Action            `json:"action"`
	Event  map[string]string `json:"event"`
}

func (e Executor) Handle(ctx context.Context, j sqlite.Job) error {
	if j.Kind != "workflow.action" {
		return fmt.Errorf("unsupported job %s", j.Kind)
	}
	var p actionJob
	if err := json.Unmarshal([]byte(j.Payload), &p); err != nil {
		return err
	}
	switch p.Action.Type {
	case "send_message", "notify_staff":
		channel := p.Event["channel_id"]
		content := p.Event["message"]
		if content == "" {
			content = "PelagornisBot workflow event"
		}
		_, err := e.Delivery.Send(ctx, channel, content)
		return err
	case "delay", "wait_for_approval":
		return nil
	case "add_role", "remove_role", "publish_panel", "create_ticket", "save_faq_candidate", "award_reputation":
		return nil
	default:
		return fmt.Errorf("disallowed action")
	}
}
