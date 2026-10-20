# PelagornisBot Lite

Discord **외부 커뮤니티** 전용 봇. **Go** · **Discord only** · **월 $0**.

**확정 스택:** [docs/DECISIONS.md](docs/DECISIONS.md)

## 구현 상태

| 모듈 | 상태 |
|------|------|
| Config (YAML) | ✓ |
| Store (JSON cursor) | ✓ |
| `/rules`, 환영, 첫 메시지 | ✓ |
| RSS 홍보 (X + YouTube → 한 채널) | ✓ |
| Moderation, FAQ, `/announce`, Admin | TODO |

## 요구 사항

- Go 1.22+
- `config/config.yaml` ([free-tier](config/profiles/free-tier.yaml) 복사)
- `.env` — `DISCORD_BOT_TOKEN`

### Discord Bot — Privileged Intents (필수)

[Developer Portal](https://discord.com/developers/applications) → 앱 → **Bot** → **Privileged Gateway Intents**:

| Intent | 용도 |
|--------|------|
| **SERVER MEMBERS INTENT** | 가입 환영 (`GuildMemberAdd`) |
| **MESSAGE CONTENT INTENT** | 채팅 모더·FAQ·첫 메시지 |

끈 상태로 실행하면 `4014: Disallowed intent(s)` 가 납니다.

`.env`의 `DISCORD_GUILD_ID`와 `config.yaml`의 `guild_id`는 **같은 서버 ID**여야 합니다.

## 실행

```bash
cp config/profiles/free-tier.yaml config/config.yaml
# config.yaml · .env 편집

go mod tidy
make build
make run
# 또는: go run ./cmd/pelagornis
```

## 프로젝트 구조

```
cmd/pelagornis/          # main
internal/
  bot/                   # discordgo, 핸들러
  config/                # YAML
  promotion/             # RSS + cron
  store/                 # data/*.json
config/
docs/
deploy/pelagornis.service
```

## 문서

| 문서 | 설명 |
|------|------|
| [docs/SPEC.md](docs/SPEC.md) | 기능 스펙 |
| [docs/DECISIONS.md](docs/DECISIONS.md) | 확정 기술 선택 |
| [docs/FREE_TIER.md](docs/FREE_TIER.md) | $0 운영 |
| [docs/IMPLEMENTATION_PLAN.md](docs/IMPLEMENTATION_PLAN.md) | 구현 체크리스트 |

## 비용

호스팅(집 PC 또는 Oracle Free VM) + RSS + Discord = **월 $0**. DB·유료 API 없음.
