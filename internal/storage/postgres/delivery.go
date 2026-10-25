package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ClaimDelivery(c context.Context, guild, item, channel string) (string, bool, error) {
	var id string
	e := s.Pool.QueryRow(c, `insert into feed_deliveries(guild_id,feed_item_id,channel_id,status) values($1,$2,$3,'pending') on conflict(guild_id,feed_item_id,channel_id) do nothing returning id`, guild, item, channel).Scan(&id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return "", false, nil
		}
		return "", false, e
	}
	return id, true, nil
}
func (s *Store) CompleteDelivery(c context.Context, id, message string, err error) error {
	if err == nil {
		_, e := s.Pool.Exec(c, `update feed_deliveries set status='posted',message_id=$2,attempts=attempts+1 where id=$1`, id, message)
		return e
	}
	_, e := s.Pool.Exec(c, `update feed_deliveries set status=case when attempts>=4 then 'failed' else 'pending' end,attempts=attempts+1 where id=$1`, id)
	return e
}
