package reputation

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Award struct {
	Guild, From, To, Key, Reason string
	Points                       int
	At                           time.Time
}
type Service struct{ DB *sql.DB }

func (s Service) Award(c context.Context, a Award) error {
	if e := a.Validate(); e != nil {
		return e
	}
	var n int
	if e := s.DB.QueryRowContext(c, `select count(*) from reputation_ledger where guild_id=? and from_user=? and date(created_at)=date('now')`, a.Guild, a.From).Scan(&n); e != nil {
		return e
	}
	if n >= 10 {
		return fmt.Errorf("daily reputation limit reached")
	}
	_, e := s.DB.ExecContext(c, `insert into reputation_ledger(id,guild_id,from_user,to_user,points,reason,event_key) values(lower(hex(randomblob(16))),?,?,?,?,?,?)`, a.Guild, a.From, a.To, a.Points, a.Reason, a.Key)
	return e
}

func (a Award) Validate() error {
	if a.From == a.To {
		return fmt.Errorf("self award denied")
	}
	if a.Points < 1 || a.Points > 10 {
		return fmt.Errorf("invalid points")
	}
	if a.Guild == "" || a.Key == "" {
		return fmt.Errorf("guild and event key required")
	}
	return nil
}
