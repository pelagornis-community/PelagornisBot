package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	p.Config().MaxConns = 8
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if e = p.Ping(c); e != nil {
		p.Close()
		return nil, e
	}
	return &Store{p}, nil
}
func (s *Store) Close() { s.Pool.Close() }
func (s *Store) EnsureGuild(ctx context.Context, id, locale string) error {
	_, e := s.Pool.Exec(ctx, `insert into guilds(id,locale,config_json) values($1,$2,'{}') on conflict(id) do nothing`, id, locale)
	return e
}
func (s *Store) IsGuildAdmin(ctx context.Context, guild, user string) (bool, error) {
	var ok bool
	e := s.Pool.QueryRow(ctx, `select exists(select 1 from guild_roles where guild_id=$1 and role_kind='admin' and role_id=$2)`, guild, user).Scan(&ok)
	return ok, e
}
func (s *Store) ClaimJobs(ctx context.Context, n int) ([]Job, error) {
	rows, e := s.Pool.Query(ctx, `with c as (select id from jobs where status='pending' and available_at<=now() and (lease_until is null or lease_until<now()) order by available_at for update skip locked limit $1) update jobs j set status='running',lease_until=now()+interval '5 minutes',attempts=attempts+1 from c where j.id=c.id returning j.id,j.guild_id,j.kind,j.payload_json,j.idempotency_key,j.attempts`, n)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var r []Job
	for rows.Next() {
		var j Job
		if e := rows.Scan(&j.ID, &j.GuildID, &j.Kind, &j.Payload, &j.Key, &j.Attempts); e != nil {
			return nil, e
		}
		r = append(r, j)
	}
	return r, rows.Err()
}

type Job struct {
	ID, GuildID, Kind, Payload, Key string
	Attempts                        int
}

func (s *Store) FinishJob(ctx context.Context, id string, err error) error {
	if err == nil {
		_, e := s.Pool.Exec(ctx, `update jobs set status='done',lease_until=null where id=$1`, id)
		return e
	}
	_, e := s.Pool.Exec(ctx, `update jobs set status=case when attempts>=5 then 'dead' else 'pending' end,available_at=now()+(least(attempts,5)*interval '30 seconds'),lease_until=null,last_error=$2 where id=$1`, id, err.Error())
	return e
}
func (s *Store) InsertJob(ctx context.Context, guild, kind, payload, key string) error {
	_, e := s.Pool.Exec(ctx, `insert into jobs(id,guild_id,kind,payload_json,idempotency_key,status,available_at) values(gen_random_uuid(),$1,$2,$3::jsonb,$4,'pending',now()) on conflict(idempotency_key) do nothing`, guild, kind, payload, key)
	return e
}
func (s *Store) Audit(ctx context.Context, guild, actor, action, target string) error {
	_, e := s.Pool.Exec(ctx, `insert into audit_logs(id,guild_id,actor_id,action,target_type,target_id,metadata_json) values(gen_random_uuid(),$1,$2,$3,'resource',$4,'{}')`, guild, actor, action, target)
	return e
}
func (s *Store) Role(ctx context.Context, guild, kind string) (string, error) {
	var id string
	err := s.Pool.QueryRow(ctx, `select role_id from guild_roles where guild_id=$1 and role_kind=$2`, guild, kind).Scan(&id)
	return id, err
}
func (s *Store) SetRole(ctx context.Context, guild, kind, id string) error {
	_, err := s.Pool.Exec(ctx, `insert into guild_roles(guild_id,role_kind,role_id) values($1,$2,$3) on conflict(guild_id,role_kind) do update set role_id=excluded.role_id`, guild, kind, id)
	return err
}
func (s *Store) IncrementMetric(ctx context.Context, guild, key, dimension string) error {
	_, e := s.Pool.Exec(ctx, `insert into daily_metrics(guild_id,day,metric_key,dimension,value) values($1,current_date,$2,$3,1) on conflict(guild_id,day,metric_key,dimension) do update set value=daily_metrics.value+1`, guild, key, dimension)
	return e
}

var _ = fmt.Sprintf
