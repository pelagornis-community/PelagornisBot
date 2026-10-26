package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
)

type Runner struct{ Store *sqlite.Store }
type Event struct {
	GuildID, Key, Trigger string
	Data                  map[string]string
}

func (r Runner) Dispatch(c context.Context, e Event) error {
	rows, err := r.Store.DB.QueryContext(c, `select id,definition_json from workflows where guild_id=? and enabled=1`, e.GuildID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			return err
		}
		var d Definition
		if err = json.Unmarshal(raw, &d); err != nil || d.Validate() != nil || d.Trigger != e.Trigger || !Matches(d.Conditions, e.Data) {
			continue
		}
		tag := id + ":" + e.Key
		_, err = r.Store.DB.ExecContext(c, `insert into workflow_runs(id,guild_id,workflow_id,event_key,status) values(lower(hex(randomblob(16))),?,?,?,'pending') on conflict(workflow_id,event_key) do nothing`, e.GuildID, id, e.Key)
		if err != nil {
			return err
		}
		for n, a := range d.Actions {
			payload, _ := json.Marshal(map[string]any{"workflow_id": id, "action": a, "event": e.Data})
			if err = r.Store.InsertJob(c, e.GuildID, "workflow.action", string(payload), fmt.Sprintf("%s:%d", tag, n)); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}
