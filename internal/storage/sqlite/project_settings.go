package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// ProjectCategory returns the parent category used for staff-only project
// rooms. sql.ErrNoRows means project automation has not been configured yet.
func (s *Store) ProjectCategory(ctx context.Context, guild string) (string, error) {
	var id string
	err := s.DB.QueryRowContext(ctx, `select category_id from project_automation_settings where guild_id=?`, guild).Scan(&id)
	return id, err
}

func (s *Store) SetProjectCategory(ctx context.Context, guild, category string) error {
	_, err := s.DB.ExecContext(ctx, `insert into project_automation_settings(guild_id,category_id) values(?,?) on conflict(guild_id) do update set category_id=excluded.category_id,updated_at=current_timestamp`, guild, category)
	return err
}

// IsConfigured is a small helper for dashboard rendering. It deliberately
// treats a missing row as an unset value while surfacing real database errors.
func IsConfigured(value string, err error) (string, error) {
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) GitHubNotificationChannel(ctx context.Context, guild, kind string) (string, error) {
	if kind != "security" && kind != "audit" {
		return "", fmt.Errorf("unknown GitHub notification channel %q", kind)
	}
	var id string
	err := s.DB.QueryRowContext(ctx, `select channel_id from github_notification_channels where guild_id=? and channel_kind=?`, guild, kind).Scan(&id)
	return id, err
}

func (s *Store) SetGitHubNotificationChannel(ctx context.Context, guild, kind, channel string) error {
	if kind != "security" && kind != "audit" {
		return fmt.Errorf("unknown GitHub notification channel %q", kind)
	}
	_, err := s.DB.ExecContext(ctx, `insert into github_notification_channels(guild_id,channel_kind,channel_id) values(?,?,?) on conflict(guild_id,channel_kind) do update set channel_id=excluded.channel_id,updated_at=current_timestamp`, guild, kind, channel)
	return err
}
