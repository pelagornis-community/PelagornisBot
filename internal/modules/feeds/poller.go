package feeds

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	githubapi "github.com/pelagornis/pelagornis-bot/internal/modules/github"
	"github.com/pelagornis/pelagornis-bot/internal/storage/sqlite"
	"log/slog"
	"strings"
	"time"
)

type Source struct {
	ID, GuildID, Kind, URL, Repo, ChannelID, ETag, LastModified, LastChecked string
	Interval                                                                 int
}
type Poller struct {
	Store  *sqlite.Store
	Client *Client
}

func (p Poller) EnqueueDue(c context.Context) error {
	rows, e := p.Store.DB.QueryContext(c, `select id,guild_id from feed_sources where enabled=1 and (last_checked_at is null or datetime(last_checked_at,'+' || interval_seconds || ' seconds')<=datetime('now')) limit 20`)
	return p.enqueueRows(c, rows, e, "due:"+time.Now().UTC().Format("200601021504"))
}

// EnqueueAll asks every active source for an immediate catch-up pass. It is
// used only at process startup; normal operation remains interval based.
func (p Poller) EnqueueAll(c context.Context) error {
	rows, e := p.Store.DB.QueryContext(c, `select id,guild_id from feed_sources where enabled=1`)
	return p.enqueueRows(c, rows, e, "startup:"+time.Now().UTC().Format("200601021504"))
}

func (p Poller) enqueueRows(c context.Context, rows *sql.Rows, e error, keyPrefix string) error {
	if e != nil {
		return e
	}
	// Store uses one SQLite connection. Read the entire result set and close it
	// before inserting jobs, otherwise InsertJob waits forever for this Rows
	// cursor to release the only connection.
	type sourceRef struct{ id, guild string }
	var sources []sourceRef
	for rows.Next() {
		var id, guild string
		if e = rows.Scan(&id, &guild); e != nil {
			rows.Close()
			return e
		}
		sources = append(sources, sourceRef{id: id, guild: guild})
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return e
	}
	if e = rows.Close(); e != nil {
		return e
	}
	for _, source := range sources {
		if e = p.enqueue(c, source.guild, source.id, keyPrefix); e != nil {
			return e
		}
	}
	return nil
}
func (p Poller) enqueue(c context.Context, guild, sourceID, keySuffix string) error {
	payload, err := json.Marshal(map[string]string{"source_id": sourceID})
	if err != nil {
		return err
	}
	return p.Store.InsertJob(c, guild, "feed.poll", string(payload), "feed.poll:"+sourceID+":"+keySuffix)
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
	e := p.Store.DB.QueryRowContext(c, `select id,guild_id,kind,coalesce(url,''),coalesce(repo,''),channel_id,coalesce(etag,''),coalesce(last_modified,''),coalesce(last_checked_at,''),interval_seconds from feed_sources where id=? and guild_id=? and enabled=1`, x.SourceID, j.GuildID).Scan(&s.ID, &s.GuildID, &s.Kind, &s.URL, &s.Repo, &s.ChannelID, &s.ETag, &s.LastModified, &s.LastChecked, &s.Interval)
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
	// Older installations may contain duplicate registrations for a repository.
	// Keep the first active source as the delivery owner so those rows cannot
	// create duplicate Discord announcements.
	var primaryID string
	if err := p.Store.DB.QueryRowContext(c, `select id from feed_sources where guild_id=? and kind='github_release' and repo=? and enabled=1 order by id limit 1`, s.GuildID, s.Repo).Scan(&primaryID); err != nil {
		return err
	}
	if primaryID != s.ID {
		slog.Info("skipping duplicate GitHub release source", "source_id", s.ID, "primary_source_id", primaryID, "repo", s.Repo)
		return nil
	}
	slog.Info("checking GitHub release", "source_id", s.ID, "repo", s.Repo)
	releases, etag, unchanged, err := githubapi.New().Releases(c, s.Repo, s.ETag)
	if err != nil {
		slog.Warn("GitHub release check failed", "source_id", s.ID, "repo", s.Repo, "error", err)
		return err
	}
	_, _ = p.Store.DB.ExecContext(c, `update feed_sources set etag=?,last_checked_at=current_timestamp where id=?`, etag, s.ID)
	if s.LastChecked == "" {
		slog.Info("GitHub release baseline established", "source_id", s.ID, "repo", s.Repo)
		return nil
	}
	if unchanged {
		slog.Info("GitHub releases unchanged", "source_id", s.ID)
		return nil
	}
	checkedAt, parseErr := time.Parse("2006-01-02 15:04:05", s.LastChecked)
	if parseErr != nil {
		return fmt.Errorf("parse release checkpoint: %w", parseErr)
	}
	// GitHub returns newest first. Queue oldest first so a burst of package
	// releases reads naturally in Discord; delivery records make repeats safe.
	for n := len(releases) - 1; n >= 0; n-- {
		r := releases[n]
		if r.Draft || r.Prerelease {
			continue
		}
		if !r.Published.After(checkedAt.UTC()) {
			continue
		}
		guid := fmt.Sprintf("github-release:%d", r.ID)
		var itemID string
		err = p.Store.DB.QueryRowContext(c, `insert into feed_items(id,source_id,guid,canonical_url,hash,published_at) values(lower(hex(randomblob(16))),?,?,?,?,?) on conflict(source_id,guid) do update set guid=excluded.guid returning id`, s.ID, guid, r.URL, fmt.Sprintf("%x", sha256.Sum256([]byte(guid+r.URL))), r.Published.UTC().Format(time.RFC3339)).Scan(&itemID)
		if err != nil {
			return err
		}
		deliveryID, claimed, err := p.Store.ClaimDelivery(c, s.GuildID, itemID, s.ChannelID)
		if err != nil {
			return err
		}
		if !claimed {
			slog.Info("GitHub release already delivered", "source_id", s.ID, "release_id", r.ID)
			continue
		}
		name := r.Name
		if name == "" {
			name = r.Tag
		}
		notes := strings.TrimSpace(r.Body)
		if notes == "" {
			notes = "_No release notes were provided._"
		}
		imageURL, imageErr := githubapi.New().ReleaseImage(c, r.URL)
		if imageErr != nil {
			slog.Warn("GitHub release image unavailable", "release_id", r.ID, "error", imageErr)
		}
		payload, _ := json.Marshal(map[string]string{"delivery_id": deliveryID, "channel_id": s.ChannelID, "content": "GitHub release: [" + s.Repo + "] · " + name + "\n" + notes + "\n" + r.URL, "image_url": imageURL})
		if err = p.Store.InsertJob(c, s.GuildID, "feed.delivery", string(payload), "feed.delivery:"+deliveryID); err != nil {
			return err
		}
		slog.Info("GitHub release delivery queued", "source_id", s.ID, "release_id", r.ID, "channel_id", s.ChannelID)
	}
	return nil
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
	var existing string
	err := p.Store.DB.QueryRowContext(c, `select id from feed_sources where guild_id=? and kind='github_release' and repo=? and enabled=1 limit 1`, guild, repo).Scan(&existing)
	if err == nil {
		return "", fmt.Errorf("repository is already registered (source %s)", existing)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	var id string
	e := p.Store.DB.QueryRowContext(c, `insert into feed_sources(id,guild_id,kind,repo,channel_id,interval_seconds,last_checked_at) values(lower(hex(randomblob(16))),?,'github_release',?,?,?,current_timestamp) returning id`, guild, repo, channel, interval).Scan(&id)
	if e != nil {
		return "", e
	}
	// A newly configured release source should be checked immediately; it must
	// not wait for the one-minute due-work scheduler tick.
	if e = p.enqueue(c, guild, id, fmt.Sprintf("registered:%d", time.Now().UnixNano())); e != nil {
		return "", e
	}
	return id, nil
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
