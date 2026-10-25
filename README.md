# CommunityOS

Multi-guild Discord community operations bot for Oracle Always Free ARM64. This repository supersedes the legacy Pelagornis Lite configuration.

## Quick start

1. Copy `.env.example` to `.env`, set the Discord token, application ID and PostgreSQL URL, then protect it with `chmod 600 .env`.
2. Start PostgreSQL with `docker compose -f deployments/compose.yaml up -d db`.
3. Run `make test`, `make lint`, then `make build-arm64`.

Live Discord installation requires an operator-owned bot token and enabled Guild Members intent. See [setup](docs/SETUP.md) and [operations](docs/OPERATIONS.md).
