package sqlite

import (
	"context"
	"database/sql"
)

func (s *Store) ClaimDelivery(c context.Context, guild, item, channel string) (id string, claimed bool, e error) {
	e = s.DB.QueryRowContext(c, `insert into feed_deliveries(id,guild_id,feed_item_id,channel_id,status) values(lower(hex(randomblob(16))),?,?,?,'pending') on conflict(guild_id,feed_item_id,channel_id) do nothing returning id`, guild, item, channel).Scan(&id)
	if e == sql.ErrNoRows {
		return "", false, nil
	}
	return id, e == nil, e
}

func (s *Store) CompleteDelivery(c context.Context, id, message string, deliveryErr error) error {
	if deliveryErr == nil {
		_, e := s.DB.ExecContext(c, `update feed_deliveries set status='posted',message_id=?,attempts=attempts+1 where id=?`, message, id)
		return e
	}
	_, e := s.DB.ExecContext(c, `update feed_deliveries set status=case when attempts>=4 then 'failed' else 'pending' end,attempts=attempts+1 where id=?`, id)
	return e
}
