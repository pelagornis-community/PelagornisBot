package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is intentionally single-writer. SQLite WAL mode keeps reads responsive
// while preserving reliable job leasing on the small single-instance deployment.
type Store struct{ DB *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("SQLITE_PATH is required")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0o750); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if e = db.PingContext(c); e != nil {
		_ = db.Close()
		return nil, e
	}
	return &Store{DB: db}, nil
}

func (s *Store) Close() { _ = s.DB.Close() }

func (s *Store) EnsureGuild(ctx context.Context, id, locale string) error {
	_, e := s.DB.ExecContext(ctx, `insert into guilds(id,locale,config_json) values(?,?, '{}') on conflict(id) do nothing`, id, locale)
	return e
}

func (s *Store) ClaimJobs(ctx context.Context, n int) ([]Job, error) {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `select id,guild_id,kind,payload_json,idempotency_key,attempts from jobs where status='pending' and available_at<=datetime('now') and (lease_until is null or lease_until<datetime('now')) order by available_at limit ?`, n)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var j Job
		if e = rows.Scan(&j.ID, &j.GuildID, &j.Kind, &j.Payload, &j.Key, &j.Attempts); e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, `update jobs set status='running',lease_until=datetime('now','+5 minutes'),attempts=attempts+1 where id=? and status='pending'`, j.ID); e != nil {
			return nil, e
		}
		j.Attempts++
		jobs = append(jobs, j)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return jobs, tx.Commit()
}

type Job struct {
	ID, GuildID, Kind, Payload, Key string
	Attempts                        int
}

func (s *Store) FinishJob(ctx context.Context, id string, jobErr error) error {
	if jobErr == nil {
		_, e := s.DB.ExecContext(ctx, `update jobs set status='done',lease_until=null where id=?`, id)
		return e
	}
	_, e := s.DB.ExecContext(ctx, `update jobs set status=case when attempts>=5 then 'dead' else 'pending' end,available_at=datetime('now','+' || (case when attempts<5 then attempts else 5 end)*30 || ' seconds'),lease_until=null,last_error=? where id=?`, jobErr.Error(), id)
	return e
}

func (s *Store) InsertJob(ctx context.Context, guild, kind, payload, key string) error {
	_, e := s.DB.ExecContext(ctx, `insert into jobs(id,guild_id,kind,payload_json,idempotency_key,status,available_at) values(lower(hex(randomblob(16))),?,?,?,?, 'pending',datetime('now')) on conflict(idempotency_key) do nothing`, guild, kind, payload, key)
	return e
}

func (s *Store) Audit(ctx context.Context, guild, actor, action, target string) error {
	_, e := s.DB.ExecContext(ctx, `insert into audit_logs(id,guild_id,actor_id,action,target_type,target_id,metadata_json) values(lower(hex(randomblob(16))),?,?,?,'resource',?,'{}')`, guild, actor, action, target)
	return e
}

func (s *Store) Role(ctx context.Context, guild, kind string) (string, error) {
	var id string
	e := s.DB.QueryRowContext(ctx, `select role_id from guild_roles where guild_id=? and role_kind=?`, guild, kind).Scan(&id)
	return id, e
}

func (s *Store) SetRole(ctx context.Context, guild, kind, id string) error {
	_, e := s.DB.ExecContext(ctx, `insert into guild_roles(guild_id,role_kind,role_id) values(?,?,?) on conflict(guild_id,role_kind) do update set role_id=excluded.role_id`, guild, kind, id)
	return e
}

func (s *Store) IncrementMetric(ctx context.Context, guild, key, dimension string) error {
	_, e := s.DB.ExecContext(ctx, `insert into daily_metrics(guild_id,day,metric_key,dimension,value) values(?,date('now'),?,?,1) on conflict(guild_id,day,metric_key,dimension) do update set value=daily_metrics.value+1`, guild, key, dimension)
	return e
}
