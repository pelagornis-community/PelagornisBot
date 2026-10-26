package faq

import (
	"context"
	"database/sql"
)

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db} }
func (s *Service) Search(c context.Context, guild, q string) ([]string, error) {
	r, e := s.db.QueryContext(c, `select question||char(10)||answer from faq_entries where guild_id=? and status='published' and (lower(question) like '%'||lower(?)||'%' or lower(answer) like '%'||lower(?)||'%') order by question limit 20`, guild, q, q)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	var out []string
	for r.Next() {
		var x string
		if e = r.Scan(&x); e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	return out, r.Err()
}
