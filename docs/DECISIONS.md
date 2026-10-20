# PelagornisBot Lite — 확정 기술 결정

**상태:** 확정 (비용 최소 $0)  
**기준:** 24/7 봇 + 유료 API·DB 없음 + 운영 단순

---

## 1. 호스팅 (확정)

| 우선순위 | 선택 | 월 비용 | 조건 |
|:--------:|------|--------|------|
| **1** | **집 PC / Mac mini / NAS** (상시 전원) | 전기만 | 이미 24/7 켜 두는 기기가 있을 때 |
| **2** | **Oracle Cloud Always Free** (ARM VM 1대) | $0 | 집에 상시 기기가 없을 때 |

**이 레포의 기본 배포 타깃:** Oracle VM (집 PC는 1순위로 동일 구성만 복사).

**사용하지 않음:** Firebase, Render 무료, Railway, Vercel, Fly(장기 $0 비보장), 유료 VPS.

### Oracle VM 스펙 (무료)

- Ubuntu 22.04 LTS (aarch64)
- 1 OCPU / 1GB RAM 이상 (가능하면 2GB)
- 프로세스: `systemd` 1유닛 + (선택) RSSHub Docker 1컨테이너

### 집 PC일 때

- 동일 바이너리·동일 `config.yaml`
- 프로세스 관리: **systemd** (권장) 또는 `nohup`

---

## 2. 런타임·라이브러리 (확정)

| 항목 | 선택 |
|------|------|
| Language | **Go 1.22+** |
| Discord | **discordgo** (`github.com/bwmarrin/discordgo`) |
| RSS | **gofeed** (`github.com/mmcdole/gofeed`) |
| 스케줄 | **robfig/cron** (30분) |
| 설정 | **YAML** (`gopkg.in/yaml.v3`) + Go `validate()` |
| env | **godotenv** — `DISCORD_BOT_TOKEN` 등 최소만 |
| 배포 | **단일 바이너리** (`bin/pelagornis`) — Node 런타임 불필요 |

**Go를 쓰는 이유 (Lite):** Oracle 1GB RAM에서 메모리 적음 · 의존성 1개 파일 · $0 호스팅에 유리.

**설치하지 않음:** Redis, PostgreSQL, SQLite(MVP), Node.js, Firebase SDK.

---

## 3. 외부 연동 (확정)

| 서비스 | 방식 | 비용 |
|--------|------|------|
| Discord | Bot Gateway | $0 |
| X | **RSS** via **RSSHub** (같은 VM, `127.0.0.1:1200`) | $0 |
| YouTube | **Atom RSS** (`feeds/videos.xml?channel_id=`) | $0 |
| X API / YouTube Data API | **사용 안 함** | — |

### X RSS URL (확정 형태)

```
http://127.0.0.1:1200/twitter/user/{handle}
```

RSSHub는 봇과 **같은 VM**에서 Docker 1개로 실행.

### YouTube RSS URL (확정 형태)

```
https://www.youtube.com/feeds/videos.xml?channel_id=UCxxxxxxxx
```

`promotion.youtube.use_api: false` 고정.

---

## 4. 데이터·상태 (확정)

| 데이터 | 저장소 |
|--------|--------|
| 홍보 cursor | `data/cursors.json` |
| 첫 메시지 플래그 | `data/first_message.json` |
| 피드백 | Discord `#feedback` 채널 |
| 규칙·FAQ·금지어 | `config/config.yaml` (+ git) |

**DB 없음** — v1.1 이후에도 트래픽 보고 SQLite만 검토.

---

## 5. Discord·운영 (확정)

| 항목 | 값 |
|------|-----|
| 플랫폼 | **Discord only** |
| 홍보 채널 | **1개** (`#updates`) |
| RSS 폴링 | **30분** (`poll_minutes: 30`) |
| 피드백 DB | `store_db: false` |
| Intents | `Guilds`, `GuildMembers`, `GuildMessages`, `MessageContent` |

---

## 6. 배포 방식 (확정)

```
git clone → go build -o bin/pelagornis ./cmd/pelagornis
config/config.yaml (free-tier 기반)
.env (DISCORD_BOT_TOKEN만)
systemd enable pelagornis-bot
docker run RSSHub (선택, X용)
```

- 봇 본체는 **Docker 필수 아님** (바이너리 직접 실행)
- CI/CD·Kubernetes·Firebase **사용 안 함**

---

## 7. 환경 변수 (확정 — 이것만)

```bash
DISCORD_BOT_TOKEN=      # 필수
DISCORD_CLIENT_ID=      # slash 등록
DISCORD_GUILD_ID=       # 개발 시
```

그 외 `YOUTUBE_API_KEY`, `REDIS_URL`, `DATABASE_URL` — **설정하지 않음**.

---

## 8. 비용 요약

| 항목 | 월 비용 |
|------|--------|
| 호스팅 (Oracle 또는 집 PC) | **$0** |
| Discord / RSS / YouTube RSS | **$0** |
| API 키 | **$0** |
| DB | **$0** |
| **합계** | **$0** (집 PC는 전기만) |

---

## 9. 다음 액션 (구현 순서)

1. Discord Developer Portal — 봇 생성, Intent·권한 설정  
2. 호스팅: 집 PC **또는** Oracle VM 생성  
3. `cp config/profiles/free-tier.yaml config/config.yaml` 채우기  
4. VM에 RSSHub: `docker run -d --restart unless-stopped -p 1200:1200 diygod/rsshub`  
5. [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) Phase 1 코드 구현  
6. `systemd` 또는 `pm2`로 상시 실행  

---

*이 문서와 충돌하는 다른 문서가 있으면 **본 문서가 우선**한다.*
