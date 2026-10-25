package tickets

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db} }
func (s *Service) Create(c context.Context, guild, owner, channel string) (string, error) {
	var id string
	e := s.db.QueryRow(c, `insert into tickets(guild_id,owner_id,channel_id,status) values($1,$2,$3,'open') returning id`, guild, owner, channel).Scan(&id)
	return id, e
}
func (s *Service) Close(c context.Context, guild, id, actor string) error {
	ct, e := s.db.Exec(c, `update tickets set status='closed',closed_at=now() where id=$1 and guild_id=$2 and status in ('open','claimed')`, id, guild)
	if e != nil {
		return e
	}
	if ct.RowsAffected() != 1 {
		return fmt.Errorf("ticket not open in this guild")
	}
	return nil
}
