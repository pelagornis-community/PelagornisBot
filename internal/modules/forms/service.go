package forms

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db} }

type Form struct {
	ID, GuildID, Slug, Status string
	Version                   int
	Schema                    Schema
}
type Submission struct {
	ID, GuildID, FormID, UserID, Status string
	ReviewReason                        string
}

func (s *Service) Create(c context.Context, guild, slug string, schema Schema) (Form, error) {
	if e := schema.Validate(); e != nil {
		return Form{}, e
	}
	raw, e := json.Marshal(schema)
	if e != nil {
		return Form{}, e
	}
	f := Form{GuildID: guild, Slug: slug, Status: "draft", Version: 1}
	e = s.db.QueryRowContext(c, `insert into forms(id,guild_id,slug,version,status,schema_json) values(lower(hex(randomblob(16))),?,?,1,'draft',?) returning id`, guild, slug, string(raw)).Scan(&f.ID)
	return f, e
}
func (s *Service) Published(c context.Context, guild, id string) (Form, error) {
	var f Form
	var raw []byte
	e := s.db.QueryRowContext(c, `select id,guild_id,slug,version,status,schema_json from forms where id=? and guild_id=? and status='published'`, id, guild).Scan(&f.ID, &f.GuildID, &f.Slug, &f.Version, &f.Status, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &f.Schema)
	}
	return f, e
}
func (s *Service) Publish(c context.Context, guild, id string) error {
	ct, e := s.db.ExecContext(c, `update forms set status='published' where id=? and guild_id=? and status='draft'`, id, guild)
	if e != nil {
		return e
	}
	n, e := ct.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return fmt.Errorf("form is not a draft in this guild")
	}
	return nil
}
func (s *Service) Submit(c context.Context, f Form, user string, a map[string]string) (string, error) {
	if e := ValidateAnswers(f.Schema, a); e != nil {
		return "", e
	}
	raw, e := json.Marshal(a)
	if e != nil {
		return "", e
	}
	var id string
	e = s.db.QueryRowContext(c, `insert into form_submissions(id,guild_id,form_id,form_version,user_id,answers_json,status) values(lower(hex(randomblob(16))),?,?,?,?,?,'pending') returning id`, f.GuildID, f.ID, f.Version, user, string(raw)).Scan(&id)
	return id, e
}
func (s *Service) SetReviewChannel(c context.Context, guild, channel string) error {
	_, e := s.db.ExecContext(c, `insert into form_review_channels(guild_id,channel_id) values(?,?) on conflict(guild_id) do update set channel_id=excluded.channel_id,updated_at=current_timestamp`, guild, channel)
	return e
}
func (s *Service) ReviewChannel(c context.Context, guild string) (string, error) {
	var channel string
	e := s.db.QueryRowContext(c, `select channel_id from form_review_channels where guild_id=?`, guild).Scan(&channel)
	return channel, e
}
func (s *Service) SetReviewMessage(c context.Context, guild, id, message string) error {
	_, e := s.db.ExecContext(c, `update form_submissions set review_message_id=? where id=? and guild_id=? and status='pending' and review_message_id is null`, message, id, guild)
	return e
}

// Review completes only a pending submission.  A false result means another
// reviewer already completed it, which makes duplicate component deliveries safe.
func (s *Service) Review(c context.Context, guild, id, actor, status, reason string) (bool, error) {
	if status != "approved" && status != "rejected" {
		return false, fmt.Errorf("invalid review state")
	}
	if status == "rejected" && len(reason) == 0 {
		return false, fmt.Errorf("rejection reason is required")
	}
	ct, e := s.db.ExecContext(c, `update form_submissions set status=?,reviewed_by=?,review_reason=?,reviewed_at=current_timestamp where id=? and guild_id=? and status='pending'`, status, actor, reason, id, guild)
	if e != nil {
		return false, e
	}
	n, e := ct.RowsAffected()
	return n == 1, e
}
func (s *Service) Pending(c context.Context, guild, id string) (Submission, error) {
	var x Submission
	e := s.db.QueryRowContext(c, `select id,guild_id,form_id,user_id,status,coalesce(review_reason,'') from form_submissions where id=? and guild_id=?`, id, guild).Scan(&x.ID, &x.GuildID, &x.FormID, &x.UserID, &x.Status, &x.ReviewReason)
	if e == nil && x.Status != "pending" {
		e = fmt.Errorf("submission is not pending")
	}
	return x, e
}
