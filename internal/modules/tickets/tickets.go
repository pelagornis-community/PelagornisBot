package tickets

import (
	"context"
	"database/sql"
	"fmt"
)

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db} }
func (s *Service) Create(c context.Context, guild, owner, channel string) (string, error) {
	var id string
	e := s.db.QueryRowContext(c, `insert into tickets(id,guild_id,owner_id,channel_id,status) values(lower(hex(randomblob(16))),?,?,?,'open') returning id`, guild, owner, channel).Scan(&id)
	return id, e
}
func (s *Service) Close(c context.Context, guild, id, actor string) error {
	ct, e := s.db.ExecContext(c, `update tickets set status='closed',closed_at=current_timestamp where id=? and guild_id=? and status in ('open','claimed')`, id, guild)
	if e != nil {
		return e
	}
	n, e := ct.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("ticket not open in this guild")
	}
	return nil
}
func (s *Service) Reopen(c context.Context, guild, id string) error {
	ct, e := s.db.ExecContext(c, `update tickets set status='open',closed_at=null where id=? and guild_id=? and status='closed'`, id, guild)
	if e != nil {
		return e
	}
	n, e := ct.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("ticket not closed in this guild")
	}
	return nil
}
func (s *Service) Assign(c context.Context, guild, id, staff string) error {
	ct, e := s.db.ExecContext(c, `update tickets set status='claimed',assigned_to=? where id=? and guild_id=? and status in ('open','claimed')`, staff, id, guild)
	if e != nil {
		return e
	}
	n, e := ct.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("ticket not active in this guild")
	}
	return nil
}
