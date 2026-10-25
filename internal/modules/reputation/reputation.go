package reputation

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Award struct {
	Guild, From, To, Key, Reason string
	Points                       int
	At                           time.Time
}
type Service struct{ DB *pgxpool.Pool }

func (s Service) Award(c context.Context, a Award) error {
	if e := a.Validate(); e != nil {
		return e
	}
	var n int
	if e := s.DB.QueryRow(c, `select count(*) from reputation_ledger where guild_id=$1 and from_user=$2 and created_at>=date_trunc('day',now())`, a.Guild, a.From).Scan(&n); e != nil {
		return e
	}
	if n >= 10 {
		return fmt.Errorf("daily reputation limit reached")
	}
	_, e := s.DB.Exec(c, `insert into reputation_ledger(guild_id,from_user,to_user,points,reason,event_key) values($1,$2,$3,$4,$5,$6)`, a.Guild, a.From, a.To, a.Points, a.Reason, a.Key)
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
