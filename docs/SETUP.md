# Setup

Create a Discord application and bot, enable **Guild Members** only if onboarding is enabled, and install using `bot applications.commands`. Do not enable Message Content unless a separately approved feature requires it. Copy `.env.example`, keep `SQLITE_PATH` on local persistent storage, and use a 32+ byte `SESSION_SECRET` when the dashboard is enabled.

## Application review setup

Before publishing an application form, configure the two assignable roles and a private staff channel. The bot role must be higher than both configured roles.

1. Run `/setup action:pending-role role:@pending`.
2. Run `/setup action:member-role role:@member`.
3. Run `/setup action:review-channel channel:#staff-review`.
4. Create and publish the form with `/form action:create`, then `/form action:publish id:<form-id> channel:#apply`.

Each application is sent to the staff channel with signed **Approve** and **Reject** buttons. Only members with Manage Server or Administrator can use them. Rejections require a reason, which is stored with the submission and sent to the applicant by DM when available.

Run `docker compose -f deployments/compose.yaml up -d`; no public inbound port is required for Gateway mode. The compose volume retains the SQLite database. Run one PelagornisBot instance only. Enable HTTPS and set `ENABLE_GITHUB_WEBHOOK=true` only for signed GitHub webhooks.
