package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type GitHubProject struct {
	Repository, ChannelID, CIChannelID string
	Archived                           bool
}

func (s *Store) SetGitHubOrganization(ctx context.Context, guild, organization string) error {
	_, err := s.DB.ExecContext(ctx, `insert into github_webhook_settings(guild_id,organization) values(?,?) on conflict(guild_id) do update set organization=excluded.organization,updated_at=current_timestamp`, guild, organization)
	return err
}

func (s *Store) GitHubOrganization(ctx context.Context, guild string) (string, error) {
	var organization string
	err := s.DB.QueryRowContext(ctx, `select organization from github_webhook_settings where guild_id=?`, guild).Scan(&organization)
	return organization, err
}

func (s *Store) GuildForGitHubOrganization(ctx context.Context, organization string) (string, error) {
	var guild string
	err := s.DB.QueryRowContext(ctx, `select guild_id from github_webhook_settings where lower(organization)=lower(?)`, organization).Scan(&guild)
	return guild, err
}

func (s *Store) GitHubProject(ctx context.Context, guild, repository string) (GitHubProject, error) {
	var p GitHubProject
	var archived int
	err := s.DB.QueryRowContext(ctx, `select repository,channel_id,ci_channel_id,archived from github_projects where guild_id=? and repository=?`, guild, repository).Scan(&p.Repository, &p.ChannelID, &p.CIChannelID, &archived)
	p.Archived = archived != 0
	return p, err
}

func (s *Store) SaveGitHubProject(ctx context.Context, guild string, p GitHubProject) error {
	_, err := s.DB.ExecContext(ctx, `insert into github_projects(guild_id,repository,channel_id,ci_channel_id,archived) values(?,?,?,?,?) on conflict(guild_id,repository) do update set channel_id=excluded.channel_id,ci_channel_id=excluded.ci_channel_id,archived=excluded.archived,updated_at=current_timestamp`, guild, p.Repository, p.ChannelID, p.CIChannelID, p.Archived)
	return err
}

// QueueGitHubWebhook atomically records the globally unique delivery ID and
// schedules deferred processing. GitHub retries therefore cannot duplicate a
// Discord notification or create duplicate project rooms.
func (s *Store) QueueGitHubWebhook(ctx context.Context, guild, deliveryID, event string, raw []byte) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `insert into webhook_deliveries(id,guild_id,provider,delivery_id) values(lower(hex(randomblob(16))),?,'github',?) on conflict(provider,delivery_id) do nothing`, guild, deliveryID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	payload, err := json.Marshal(map[string]string{"event": event, "delivery_id": deliveryID, "payload": string(raw)})
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `insert into jobs(id,guild_id,kind,payload_json,idempotency_key,status,available_at) values(lower(hex(randomblob(16))),?,'github.webhook',?,?,'pending',datetime('now'))`, guild, string(payload), "github.webhook:"+deliveryID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func IsNoRows(err error) bool { return err == sql.ErrNoRows }
func ValidateGitHubOrganization(v string) error {
	if v == "" || len(v) > 100 {
		return fmt.Errorf("invalid GitHub organization")
	}
	return nil
}
