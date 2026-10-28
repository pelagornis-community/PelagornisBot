# PelagornisBot

PelagornisBot is a Discord community-operations bot for a small community on Oracle Always Free ARM64. It uses local SQLite storage and Discord-native dashboards: administrators call a command, then use embeds, buttons, selectors, and modals instead of memorising long argument lists.

> Run one bot instance and one worker per SQLite database. This is intentional for a small personal community, not a horizontally scaled deployment.

## Highlights

- Discord Components V2 layouts for command responses, dashboards, rules, and Welcome messages
- `/rule` server rules and `/welcome` interactive welcome panel
- Application forms with private staff review, role changes, audit records, and applicant notifications
- Private tickets with close, reopen, claim, and assign operations
- Self-service role panels with role hierarchy checks
- RSS/Atom and GitHub Release delivery with persisted deduplication

## Quick start

1. Copy `.env.example` to `.env`; set `DISCORD_BOT_TOKEN`, `DISCORD_APPLICATION_ID`, `SESSION_SECRET`, and `ENCRYPTION_KEY`. Keep the file private.
2. The default database is `data/pelagornisbot.db`; keep it on persistent local storage.
3. Start the bot:

   ```sh
   make migrate
   make run
   ```

4. In the Discord Developer Portal, enable **Server Members Intent**. The bot needs appropriate permissions for enabled features, including Manage Roles, Manage Channels, Send Messages, Embed Links, and Use Application Commands.

See [setup](docs/SETUP.md) and [operations](docs/OPERATIONS.md) for deployment and backup guidance.

## Discord usage

Administrative commands require **Manage Server**. That permission is checked again when an administrator clicks a button, chooses an item, or submits a modal.

| Command | Access | Result |
| --- | --- | --- |
| `/help` | Everyone | Embed-based usage guide |
| `/rule` | Everyone | Server rules embeds |
| `/faq` | Everyone | Opens a modal to search published FAQ entries |
| `/ticket` | Everyone | Support dashboard and private-ticket button |
| `/setup` | Staff | Review and configure onboarding, staff access, and project-automation settings |
| `/welcome` | Staff | Publishes the welcome panel in the current channel |
| `/form` | Staff | Application-form and review-channel dashboard |
| `/panel` | Staff | Ticket and role-panel dashboard |
| `/role` | Staff | Role selector and self-service panel publisher |
| `/feeds` | Staff | RSS/Atom dashboard; updates are delivered to <#1091733656277618788> |
| `/release` | Staff | GitHub Release dashboard; modal registers `owner/repository` and sends published releases to <#1091734148957339749> |
| `/stats` | Staff | Guild-scoped form, ticket, and delivery totals |
| `/workflow` | Staff | Workflow dashboard; dialog accepts a name and typed JSON definition |

Management commands deliberately have no slash-command arguments. Run the command, then complete its action through the signed dashboard controls. Release registration can be opened in any channel, but delivery is always sent to <#1091734148957339749>.

### Welcome Hero image

Place the optional Welcome Hero image at `assets/hero.png`. When `/welcome` publishes a new message, PelagornisBot uploads that file and renders it inside the same Components V2 Welcome container, above the introduction and buttons. If the file is absent, the Welcome panel still publishes without an image.

The `Read the rules` button links directly to the configured Discord rules channel, while `Get support` opens a private ticket. Re-run `/welcome` after changing the Hero image or Welcome copy; already-posted Discord messages are not modified.

### First-time setup

1. Run `/setup` and choose the staff role, member role, pending role, and private staff review channel.
2. In the same dashboard, select the private parent category for future GitHub project rooms. For this server, select <#1091726089174392833>.
3. Set **Security alerts** to <#1565011226512203806> and **GitHub audit** to <#1565011279335129248>. These remain private staff destinations for future GitHub webhook routing.
4. Run `/welcome` in the welcome channel.
5. Run `/form` to create an application form; publish it using the returned ID if applications are needed.
6. Run `/panel` in a support channel, or `/role` in a roles channel.

The configured staff role will be granted access to future private project rooms. The bot role must be above every role it manages. PelagornisBot rejects unsafe role assignments.

## Storage and operations

- `SQLITE_PATH` defaults to `data/pelagornisbot.db`.
- SQLite runs in WAL mode with a single writer connection. Do not run multiple bot instances against one database file.
- Apply embedded migrations with `make migrate`. PostgreSQL data is not imported automatically.
- Create a consistent online backup with:

  ```sh
  sqlite3 data/pelagornisbot.db ".backup '/safe/backup/pelagornisbot.db'"
  ```

Do not copy only the main database file while the bot is running; WAL mode requires a consistent SQLite backup.

## Validation and builds

```sh
make check       # format check, tests, and go vet
make smoke       # offline smoke checks and Linux ARM64 build
make build       # local binary: bin/pelagornisbot
make build-arm64 # Linux ARM64 binary
```

`make compose-config` validates Docker Compose when Docker is installed.

## Current scope

The bot deliberately focuses on a small, single-instance community. Workflow definitions can be stored, but workflow execution coverage is incomplete; do not rely on workflows for critical moderation or provisioning. FAQ currently exposes published-entry search; staff CRUD/approval UI is not yet available. The OAuth dashboard, full analytics retention system, and multi-instance deployment described in older planning documents are not complete.

## Security notes

- Never post Discord tokens, API keys, webhook secrets, or database backups in Discord, Git, or screenshots.
- Use a stable, unique `SESSION_SECRET`; signed component IDs rely on it.
- Components V2 buttons, selectors, and modal submissions re-check server-management permission for administrative actions. RSS and GitHub Release sources cannot be added by ordinary members.
- Restrict staff review channels and backups to trusted operators.
- Enable GitHub webhooks only after setting `GITHUB_WEBHOOK_SECRET`.
