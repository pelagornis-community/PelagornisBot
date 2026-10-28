-- +goose Up
-- Project automation is opt-in per Discord guild. Keeping this distinct from
-- feed sources lets an organization webhook create staff-only project rooms
-- without making a repository public by default.
create table project_automation_settings(
  guild_id text primary key references guilds(id) on delete cascade,
  category_id text not null,
  updated_at text not null default current_timestamp
);

-- +goose Down
drop table if exists project_automation_settings;
