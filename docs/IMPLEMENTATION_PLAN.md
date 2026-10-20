# PelagornisBot Lite — 구현 플랜

[SPEC.md](./SPEC.md) · [FREE_TIER.md](./FREE_TIER.md) 기준 실행 체크리스트.

---

## Phase 0: 준비 (1~2일)

- [ ] Discord Developer Portal에서 Application 생성
- [ ] Bot Token, Client ID, 개발용 Guild ID 확보
- [ ] Intents: `Guilds`, `GuildMembers`, `GuildMessages`, `MessageContent` (모더레이션·FAQ에 필요)
- [ ] Permissions: Manage Messages, Moderate Members, Send Messages, Embed Links, Pin Messages
- [ ] X → RSS 피드 URL 준비 (RSSHub 자체 호스트 권장, 또는 rss.app 등)
- [ ] YouTube → `https://www.youtube.com/feeds/videos.xml?channel_id=UC...` 확인
- [ ] `config/profiles/free-tier.yaml` → `config/config.yaml` 복사·채우기
- [ ] (무료) YouTube `use_api: false` — API 키 **발급 안 함**
- [ ] (무료) 호스팅: Oracle Always Free VM 또는 집 PC 계획 ([FREE_TIER.md](./FREE_TIER.md))
- [ ] 테스트 서버: `#updates`(홍보 **단일 채널**), `#announce`, `#mod-log` 생성
---

## Phase 1: 뼈대 (Week 1)

### 1.1 프로젝트 초기화 (Go)

```bash
go mod tidy
make build
```

- [x] `go.mod`, `cmd/pelagornis/main.go`
- [x] `internal/config` — YAML 로드·검증
- [x] `internal/store` — JSON cursor
- [x] `config/config.example.yaml` → `config/config.yaml`

### 1.2 Onboarding

- [x] `GuildMemberAdd` → welcome
- [x] `/rules` 슬래시 + Embed
- [x] 첫 메시지 리마인더 (`data/first_message.json`)

**완료 기준:** 신규 유저 가입 시 환영, `/rules` 동작.

---

## Phase 2: Moderation + Security (Week 2)

### 2.1 Moderation

- [ ] `config/moderation.yaml` 로드
- [ ] 금지어: 대소문자 무시, 단어 경계(선택)
- [ ] Rate limit: `Map<userId, timestamps[]>` + prune
- [ ] 신규 유저: `member.joinedTimestamp` vs `link_restrict_hours`
- [ ] 삭제 시 `#mod-log` Embed

### 2.2 Security (optional)

- [ ] regex 목록 순회, match 시 delete 또는 mask 안내
- [ ] mod-log 알림

**완료 기준:** 금지어·5메시지/10초·신규 링크 시나리오 수동 테스트 통과.

---

## Phase 3: Auto Promotion (Week 3) — Core

### 3.1 Promotion (Go)

- [x] `internal/promotion` — gofeed + cron
- [x] 단일 `discord_channel_id`
- [x] `data/cursors.json` — `x_rss`, `youtube_rss`
- [ ] RSSHub 연동 테스트

**완료 기준:** X·YT 각 1건 게시 후, 재시작해도 중복 없음.

---

## Phase 4: Community Features (Week 4)

- [ ] `/announce` — Mod, pin 옵션
- [ ] `/subscribe`, `/unsubscribe` — 역할 부여/제거
- [ ] FAQ — keyword match, channel cooldown
- [ ] `/feedback` — Modal → inbox 채널

**완료 기준:** FAQ “download” → 링크 응답, feedback 채널에 포스트.

---

## Phase 5: Admin + 배포 (Week 5~6)

- [ ] `/delete`, `/mute`, `/unmute`, `/pin`
- [ ] slash command guild deploy 스크립트
- [ ] `Dockerfile` + `docker-compose.yml`
- [ ] README: 설정·권한·채널 구성 가이드
- [ ] (선택) GitHub Actions: lint + typecheck

---

## 테스트 시나리오

| # | 시나리오 | 기대 결과 |
|---|----------|-----------|
| 1 | 신규 가입 | 환영 메시지 |
| 2 | 금지어 포함 메시지 | 삭제 + mod-log |
| 3 | 10초에 6메시지 | timeout |
| 4 | 가입 직후 URL | 삭제 + 안내 |
| 5 | X RSS 새 항목 | `#updates` Embed 1회 |
| 6 | YouTube RSS 새 영상 | **같은** `#updates` Embed 1회 |
| 7 | `/feedback` | inbox 채널 기록 |
| 8 | 채팅에 `sk-...` | 삭제/마스킹 + mod-log |

---

## 다음 문서 (선택)

| 문서 | 용도 |
|------|------|
| [MODULE_API.md](./MODULE_API.md) | TypeScript 인터페이스·이벤트 훅 |
| [config/config.example.yaml](../config/config.example.yaml) | 배포용 설정 템플릿 |

구현 착수 시 `config.example.yaml`과 `MODULE_API.md`를 코드와 동기화한다.
