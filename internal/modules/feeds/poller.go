package feeds

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/pelagornis/communityos/internal/storage/postgres"
	"time"
)

type Source struct {
	ID, GuildID, URL, ChannelID, ETag, LastModified string
	Interval                                        int
}
type Poller struct {
	Store  *postgres.Store
	Client *Client
}

func (p Poller) EnqueueDue(c context.Context) error {
	rows, e := p.Store.Pool.Query(c, `select id,guild_id from feed_sources where enabled and (last_checked_at is null or last_checked_at + interval_seconds * interval '1 second' <= now()) limit 20`)
	if e != nil {
		return e
	}
	defer rows.Close()
	for rows.Next() {
		var id, guild string
		if e = rows.Scan(&id, &guild); e != nil {
			return e
		}
		if e = p.Store.InsertJob(c, guild, "feed.poll", `{"source_id":"`+id+`"}`, "feed.poll:"+id+":"+time.Now().UTC().Format("200601021504")); e != nil {
			return e
		}
	}
	return rows.Err()
}
func (p Poller) Handle(c context.Context, j postgres.Job) error {
	if j.Kind != "feed.poll" {
		return fmt.Errorf("unsupported job")
	}
	var x struct {
		SourceID string `json:"source_id"`
	}
	if e := json.Unmarshal([]byte(j.Payload), &x); e != nil {
		return e
	}
	var s Source
	e := p.Store.Pool.QueryRow(c, `select id,guild_id,url,channel_id,coalesce(etag,''),coalesce(last_modified,''),interval_seconds from feed_sources where id=$1 and guild_id=$2 and enabled`, x.SourceID, j.GuildID).Scan(&s.ID, &s.GuildID, &s.URL, &s.ChannelID, &s.ETag, &s.LastModified, &s.Interval)
	if e != nil {
		return e
	}
	r, e := p.Client.Fetch(c, s.URL, s.ETag, s.LastModified)
	if e != nil {
		return e
	}
	_, _ = p.Store.Pool.Exec(c, `update feed_sources set etag=$2,last_modified=$3,last_checked_at=now() where id=$1`, s.ID, r.ETag, r.LastModified)
	if r.NotModified {
		return nil
	}
	for _, item := range r.Items {
		var itemID string
		h := fmt.Sprintf("%x", sha256.Sum256([]byte(item.URL+item.Title)))
		e = p.Store.Pool.QueryRow(c, `insert into feed_items(source_id,guid,canonical_url,hash,published_at) values($1,$2,$3,$4,$5) on conflict(source_id,guid) do update set guid=excluded.guid returning id`, s.ID, item.GUID, item.URL, h, item.Published).Scan(&itemID)
		if e != nil {
			return e
		}
		deliveryID, claimed, e := p.Store.ClaimDelivery(c, s.GuildID, itemID, s.ChannelID)
		if e != nil {
			return e
		}
		if !claimed {
			continue
		}
		payload, _ := json.Marshal(map[string]string{"delivery_id": deliveryID, "channel_id": s.ChannelID, "content": item.Title + "\n" + item.URL})
		if e = p.Store.InsertJob(c, s.GuildID, "feed.delivery", string(payload), "feed.delivery:"+deliveryID); e != nil {
			return e
		}
	}
	return nil
}
func (p Poller) Add(c context.Context, guild, url, channel string, interval int) (string, error) {
	if interval < 300 {
		return "", fmt.Errorf("minimum feed interval is 300 seconds")
	}
	var id string
	e := p.Store.Pool.QueryRow(c, `insert into feed_sources(guild_id,kind,url,channel_id,interval_seconds) values($1,'rss',$2,$3,$4) returning id`, guild, url, channel, interval).Scan(&id)
	return id, e
}
func (p Poller) List(c context.Context, guild string) ([]Source, error) {
	rows, e := p.Store.Pool.Query(c, `select id,guild_id,url,channel_id,coalesce(etag,''),coalesce(last_modified,''),interval_seconds from feed_sources where guild_id=$1 order by id`, guild)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		if e = rows.Scan(&s.ID, &s.GuildID, &s.URL, &s.ChannelID, &s.ETag, &s.LastModified, &s.Interval); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (p Poller) Pause(c context.Context, guild, id string) error {
	ct, e := p.Store.Pool.Exec(c, `update feed_sources set enabled=false where id=$1 and guild_id=$2 and enabled`, id, guild)
	if e != nil {
		return e
	}
	if ct.RowsAffected() != 1 {
		return fmt.Errorf("feed not active in this guild")
	}
	return nil
}

var _ = pgx.ErrNoRows
