-- +goose Up
create table github_notification_channels(
  guild_id text not null references guilds(id) on delete cascade,
  channel_kind text not null check(channel_kind in ('security','audit')),
  channel_id text not null,
  updated_at text not null default current_timestamp,
  primary key(guild_id,channel_kind)
);

-- +goose Down
drop table if exists github_notification_channels;
