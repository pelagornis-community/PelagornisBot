package faq

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db} }
func (s *Service) Search(c context.Context, guild, q string) ([]string, error) {
	r, e := s.db.Query(c, `select question||E'\n'||answer from faq_entries where guild_id=$1 and status='published' and (question ilike '%'||$2||'%' or answer ilike '%'||$2||'%') order by question limit 20`, guild, q)
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
