# Setup

Create a Discord application and bot, enable **Guild Members** only if onboarding is enabled, and install using `bot applications.commands`. Do not enable Message Content unless a separately approved feature requires it. Copy `.env.example`, provide a PostgreSQL URL, and use a 32+ byte `SESSION_SECRET` when the dashboard is enabled.

Run `docker compose -f deployments/compose.yaml up -d`; no public inbound port is required for Gateway mode. Enable HTTPS and set `ENABLE_GITHUB_WEBHOOK=true` only for signed GitHub webhooks.
