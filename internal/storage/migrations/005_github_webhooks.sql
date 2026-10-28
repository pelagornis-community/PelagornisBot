-- +goose Up
create table github_webhook_settings(
  guild_id text primary key references guilds(id) on delete cascade,
  organization text not null unique,
  updated_at text not null default current_timestamp
);
create table github_projects(
  guild_id text not null references guilds(id) on delete cascade,
  repository text not null,
  channel_id text not null,
  ci_channel_id text not null,
  archived integer not null default 0,
  updated_at text not null default current_timestamp,
  primary key(guild_id,repository)
);

-- +goose Down
drop table if exists github_projects;
drop table if exists github_webhook_settings;
