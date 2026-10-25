package forms

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db} }

type Form struct {
	ID, GuildID, Slug, Status string
	Version                   int
	Schema                    Schema
}
type Submission struct{ ID, GuildID, FormID, UserID, Status string }

func (s *Service) Create(c context.Context, guild, slug string, schema Schema) (Form, error) {
	if e := schema.Validate(); e != nil {
		return Form{}, e
	}
	raw, e := json.Marshal(schema)
	if e != nil {
		return Form{}, e
	}
	f := Form{GuildID: guild, Slug: slug, Status: "draft", Version: 1}
	e = s.db.QueryRow(c, `insert into forms(guild_id,slug,version,status,schema_json) values($1,$2,1,'draft',$3) returning id`, guild, slug, raw).Scan(&f.ID)
	return f, e
}
func (s *Service) Published(c context.Context, guild, id string) (Form, error) {
	var f Form
	var raw []byte
	e := s.db.QueryRow(c, `select id,guild_id,slug,version,status,schema_json from forms where id=$1 and guild_id=$2 and status='published'`, id, guild).Scan(&f.ID, &f.GuildID, &f.Slug, &f.Version, &f.Status, &raw)
	if e == nil {
		e = json.Unmarshal(raw, &f.Schema)
	}
	return f, e
}
func (s *Service) Publish(c context.Context, guild, id string) error {
	ct, e := s.db.Exec(c, `update forms set status='published' where id=$1 and guild_id=$2 and status='draft'`, id, guild)
	if e != nil {
		return e
	}
	if ct.RowsAffected() != 1 {
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
	e = s.db.QueryRow(c, `insert into form_submissions(guild_id,form_id,form_version,user_id,answers_json,status) values($1,$2,$3,$4,$5,'pending') returning id`, f.GuildID, f.ID, f.Version, user, raw).Scan(&id)
	return id, e
}
func (s *Service) Review(c context.Context, guild, id, actor, status string) error {
	if status != "approved" && status != "rejected" {
		return fmt.Errorf("invalid review state")
	}
	ct, e := s.db.Exec(c, `update form_submissions set status=$1,reviewed_by=$2 where id=$3 and guild_id=$4 and status='pending'`, status, actor, id, guild)
	if e != nil {
		return e
	}
	if ct.RowsAffected() != 1 {
		return fmt.Errorf("submission is not pending in this guild")
	}
	return nil
}
func (s *Service) Pending(c context.Context, guild, id string) (Submission, error) {
	var x Submission
	e := s.db.QueryRow(c, `select id,guild_id,form_id,user_id,status from form_submissions where id=$1 and guild_id=$2`, id, guild).Scan(&x.ID, &x.GuildID, &x.FormID, &x.UserID, &x.Status)
	if e == nil && x.Status != "pending" {
		e = fmt.Errorf("submission is not pending")
	}
	return x, e
}
