package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pelagornis/communityos/internal/storage/postgres"
)

type Runner struct{ Store *postgres.Store }
type Event struct {
	GuildID, Key, Trigger string
	Data                  map[string]string
}

func (r Runner) Dispatch(c context.Context, e Event) error {
	rows, err := r.Store.Pool.Query(c, `select id,definition_json from workflows where guild_id=$1 and enabled=true`, e.GuildID)
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
		_, err = r.Store.Pool.Exec(c, `insert into workflow_runs(guild_id,workflow_id,event_key,status) values($1,$2,$3,'pending') on conflict(workflow_id,event_key) do nothing`, e.GuildID, id, e.Key)
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
