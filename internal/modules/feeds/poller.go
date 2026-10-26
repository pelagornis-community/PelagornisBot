package feeds

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	githubapi "github.com/pelagornis/pelagornis-bot/internal/modules/github"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"time"
)

type Source struct {
	ID, GuildID, Kind, URL, Repo, ChannelID, ETag, LastModified string
	Interval                                                    int
}
type Poller struct {
	Store  *sqlite.Store
	Client *Client
}

func (p Poller) EnqueueDue(c context.Context) error {
	rows, e := p.Store.DB.QueryContext(c, `select id,guild_id from feed_sources where enabled=1 and (last_checked_at is null or datetime(last_checked_at,'+' || interval_seconds || ' seconds')<=datetime('now')) limit 20`)
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
func (p Poller) Handle(c context.Context, j sqlite.Job) error {
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
	e := p.Store.DB.QueryRowContext(c, `select id,guild_id,kind,coalesce(url,''),coalesce(repo,''),channel_id,coalesce(etag,''),coalesce(last_modified,''),interval_seconds from feed_sources where id=? and guild_id=? and enabled=1`, x.SourceID, j.GuildID).Scan(&s.ID, &s.GuildID, &s.Kind, &s.URL, &s.Repo, &s.ChannelID, &s.ETag, &s.LastModified, &s.Interval)
	if e != nil {
		return e
	}
	if s.Kind == "github_release" {
		return p.handleRelease(c, s)
	}
	if s.Kind != "rss" {
		return fmt.Errorf("unsupported feed source kind %q", s.Kind)
	}
	r, e := p.Client.Fetch(c, s.URL, s.ETag, s.LastModified)
	if e != nil {
		return e
	}
	_, _ = p.Store.DB.ExecContext(c, `update feed_sources set etag=?,last_modified=?,last_checked_at=current_timestamp where id=?`, r.ETag, r.LastModified, s.ID)
	if r.NotModified {
		return nil
	}
	for _, item := range r.Items {
		var itemID string
		h := fmt.Sprintf("%x", sha256.Sum256([]byte(item.URL+item.Title)))
		e = p.Store.DB.QueryRowContext(c, `insert into feed_items(id,source_id,guid,canonical_url,hash,published_at) values(lower(hex(randomblob(16))),?,?,?,?,?) on conflict(source_id,guid) do update set guid=excluded.guid returning id`, s.ID, item.GUID, item.URL, h, item.Published.UTC().Format(time.RFC3339)).Scan(&itemID)
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

func (p Poller) handleRelease(c context.Context, s Source) error {
	r, etag, unchanged, err := githubapi.New().Latest(c, s.Repo, s.ETag)
	if err != nil {
		return err
	}
	_, _ = p.Store.DB.ExecContext(c, `update feed_sources set etag=?,last_checked_at=current_timestamp where id=?`, etag, s.ID)
	if unchanged || r.Draft || r.Prerelease {
		return nil
	}
	guid := fmt.Sprintf("github-release:%d", r.ID)
	var itemID string
	err = p.Store.DB.QueryRowContext(c, `insert into feed_items(id,source_id,guid,canonical_url,hash,published_at) values(lower(hex(randomblob(16))),?,?,?,?,?) on conflict(source_id,guid) do update set guid=excluded.guid returning id`, s.ID, guid, r.URL, fmt.Sprintf("%x", sha256.Sum256([]byte(guid+r.URL))), r.Published.UTC().Format(time.RFC3339)).Scan(&itemID)
	if err != nil {
		return err
	}
	deliveryID, claimed, err := p.Store.ClaimDelivery(c, s.GuildID, itemID, s.ChannelID)
	if err != nil || !claimed {
		return err
	}
	name := r.Name
	if name == "" {
		name = r.Tag
	}
	payload, _ := json.Marshal(map[string]string{"delivery_id": deliveryID, "channel_id": s.ChannelID, "content": "New GitHub release: " + name + "\n" + r.URL})
	return p.Store.InsertJob(c, s.GuildID, "feed.delivery", string(payload), "feed.delivery:"+deliveryID)
}
func (p Poller) Add(c context.Context, guild, url, channel string, interval int) (string, error) {
	if interval < 300 {
		return "", fmt.Errorf("minimum feed interval is 300 seconds")
	}
	var id string
	e := p.Store.DB.QueryRowContext(c, `insert into feed_sources(id,guild_id,kind,url,channel_id,interval_seconds) values(lower(hex(randomblob(16))),?,'rss',?,?,?) returning id`, guild, url, channel, interval).Scan(&id)
	return id, e
}
func (p Poller) AddRelease(c context.Context, guild, repo, channel string, interval int) (string, error) {
	if !githubapi.ValidRepo(repo) {
		return "", fmt.Errorf("repository must be owner/name")
	}
	if interval < 300 {
		return "", fmt.Errorf("minimum poll interval is 300 seconds")
	}
	var id string
	e := p.Store.DB.QueryRowContext(c, `insert into feed_sources(id,guild_id,kind,repo,channel_id,interval_seconds) values(lower(hex(randomblob(16))),?,'github_release',?,?,?) returning id`, guild, repo, channel, interval).Scan(&id)
	return id, e
}
func (p Poller) List(c context.Context, guild string) ([]Source, error) {
	rows, e := p.Store.DB.QueryContext(c, `select id,guild_id,kind,coalesce(url,''),coalesce(repo,''),channel_id,coalesce(etag,''),coalesce(last_modified,''),interval_seconds from feed_sources where guild_id=? order by id`, guild)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var s Source
		if e = rows.Scan(&s.ID, &s.GuildID, &s.Kind, &s.URL, &s.Repo, &s.ChannelID, &s.ETag, &s.LastModified, &s.Interval); e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (p Poller) Pause(c context.Context, guild, id string) error {
	ct, e := p.Store.DB.ExecContext(c, `update feed_sources set enabled=0 where id=? and guild_id=? and enabled=1`, id, guild)
	if e != nil {
		return e
	}
	n, e := ct.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("feed not active in this guild")
	}
	return nil
}
