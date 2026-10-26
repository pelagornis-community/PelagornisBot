-- +goose Up
create table form_review_channels(
  guild_id text primary key references guilds(id) on delete cascade,
  channel_id text not null,
  updated_at text not null default current_timestamp
);

alter table form_submissions
  add column review_reason text;
alter table form_submissions
  add column reviewed_at text;
alter table form_submissions
  add column review_message_id text;

-- +goose Down
-- SQLite migrations are append-only; columns remain on down migration.
drop table if exists form_review_channels;
