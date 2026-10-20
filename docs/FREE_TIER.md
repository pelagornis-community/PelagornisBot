# PelagornisBot Lite — 무료 플랜($0) 운영 가이드

**목표:** 월 고정 비용 **$0** (또는 전기세만)으로 24/7 봇 운영.

**확정 선택은 [DECISIONS.md](./DECISIONS.md)를 따른다.** 본 문서는 배경·비교·체크리스트용.

[SPEC.md](./SPEC.md) · [config/profiles/free-tier.yaml](../config/profiles/free-tier.yaml)

---

## 1. 한눈에 보기

| 항목 | 무료 구성 | 피할 것 |
|------|-----------|---------|
| 봇 플랫폼 | Discord | Slack, Teams |
| X 연동 | **RSS URL** | X API (유료 Tier) |
| YouTube | **Atom RSS** | Data API quota |
| DB | **없음** (JSON cursor + Discord 채널) | PostgreSQL, Redis |
| 호스팅 | **24/7 무료 VM** 또는 집 PC | Render 무료(슬립), Vercel(웹소켓 불가) |
| 인스턴스 | **1대** | 멀티 레플리카 + Redis |
| RSS 폴링 | **30분** 간격 | 5분 이하 폴링 |

---

## 2. 왜 24/7 호스팅이 필요한가

봇(discordgo)은 **Gateway WebSocket** 상시 연결이 필요합니다.

- 모더레이션·FAQ·슬래시 명령 → 봇이 **항상 켜져** 있어야 함
- Serverless(Vercel, Netlify Functions)만으로는 **풀 봇 불가**
- 홍보만 분리하려면: Cron + **Discord Webhook**(선택, §7) — 모더레이션은 여전히 상시 프로세스 필요

---

## 3. $0에 맞는 Lite 아키텍처

```
┌─────────────────────────────────────────┐
│  무료 VPS 1대 (Oracle / 집 PC / Pi)       │
│  ┌───────────────────────────────────┐  │
│  │  Go 바이너리 (pelagornis)           │  │
│  │  · discordgo Gateway              │  │
│  │  · cron (30분 RSS)                 │  │
│  │  · data/cursors.json (영속)       │  │
│  └───────────────────────────────────┘  │
│  (선택) RSSHub Docker — 같은 VM          │
└─────────────────────────────────────────┘
         │                    │
         ▼                    ▼
   Discord API          X/YT RSS (HTTP)
   (무료)               (무료)
```

**스펙 최소:** 1 vCPU, **512MB~1GB RAM**, 10GB 디스크.

---

## 4. 호스팅 옵션 비교

| 옵션 | 월 비용 | 24/7 | discordgo | 추천도 |
|------|---------|------|------------|--------|
| **Oracle Cloud Always Free** (ARM VM) | $0 | ✓ | ✓ | ★★★★★ |
| **집 PC / Raspberry Pi** | 전기만 | ✓ | ✓ | ★★★★☆ |
| **Fly.io** (크레딧/소형 VM) | $0~ | ✓ | ✓ | ★★★☆ (정책·크레딧 변동) |
| **Google Cloud e2-micro** (free tier) | $0* | ✓ | ✓ | ★★★☆ (*조건부) |
| **Render 무료** | $0 | ✗ 슬립 | ✗ 끊김 | ✗ |
| **Railway** | Trial 후 유료 | ✓ | ✓ | ✗ 장기 $0 아님 |
| **Vercel / Netlify** | $0 | ✗ | ✗ | ✗ 풀 봇 부적합 |

**실전 추천:** Oracle Always Free VM 1대 + Docker(선택) + `systemd`로 봇 자동 재시작.

---

## 5. 외부 서비스 비용

### 5.1 Discord — 무료

- 봇 계정·API 사용료 없음
- 서버 수·명령 수는 [Discord Developer 정책](https://discord.com/developers/docs) 범위 내

### 5.2 X — RSS만 ($0)

- 공식 API **사용 안 함**
- RSS 피드 URL 필요 → [RSSHub](https://docs.rsshub.app/) 자체 호스트(같은 Oracle VM)가 가장 안정적
- 공개 `rsshub.app` 인스턴스: 무료이나 **rate limit·불안정** — 소규모 커뮤니티·테스트용

### 5.3 YouTube — Atom RSS ($0)

```
https://www.youtube.com/feeds/videos.xml?channel_id=UCxxxxxxxx
```

- `use_api: false` → **API 키·quota 없음**
- 채널 ID는 YouTube 채널 페이지 URL에서 확인

### 5.4 DB / Redis — 사용 안 함 (MVP)

| 데이터 | 무료 저장 |
|--------|-----------|
| 홍보 cursor | `data/cursors.json` |
| 첫 메시지 플래그 | `data/first_message.json` |
| 피드백 | Discord `#feedback` 채널 |
| 금지어·FAQ·규칙 | `config/*.yaml` (git) |

SQLite는 디스크만 쓰므로 **여전히 $0** — 트래픽 많아지면 v1.1에서 도입 가능.

---

## 6. config — 무료 프로필

`config/profiles/free-tier.yaml` 을 기본 예시로 쓰거나, `config.yaml`에 아래를 반영:

```yaml
# 핵심: API 키 0개, DB 0개, 폴링 30분
promotion:
  poll_minutes: 30
  youtube:
    use_api: false
    rss_feed_url: "https://www.youtube.com/feeds/videos.xml?channel_id=UC..."

feedback:
  store_db: false

# env 불필요: YOUTUBE_API_KEY, REDIS_URL, DATABASE_URL
```

환경 변수는 **Discord 토큰만** 필수:

```bash
DISCORD_BOT_TOKEN=
DISCORD_CLIENT_ID=
DISCORD_GUILD_ID=   # 개발 시 slash 등록용
```

---

## 7. (선택) 홍보만 완전 분리 — VPS 없이?

모더레이션은 집 PC/Oracle에서 돌리고, **홍보만** 아래처럼 나눌 수 있음:

```
GitHub Actions (cron, public repo 무료)
  → RSS fetch
  → Discord Incoming Webhook POST
```

| 장점 | 단점 |
|------|------|
| 홍보 트래픽이 봇 VM과 분리 | Webhook URL 유출 시 보안 위험 (시크릿 관리 필수) |
| Actions 무료 한도 내 소량 poll | Embed·핀·역할 멘션 제한 |
| | **모더레이션 봇은 여전히 24/7 필요** |

**권장:** 소규모면 **단일 프로세스**가 운영·디버깅 모두 쉬움.

---

## 8. RSSHub 자체 호스트 (X RSS 안정화, $0)

Oracle VM에서:

```bash
docker run -d --name rsshub -p 1200:1200 diygod/rsshub
```

`config.yaml`:

```yaml
promotion:
  x:
    rss_feed_url: "http://127.0.0.1:1200/twitter/user/YOUR_HANDLE"
```

- 외부 공개 URL 불필요 → rate limit 회피
- VM 재부팅 시 `docker restart` policy

---

## 9. 운영 체크리스트

### 배포 전

- [ ] `use_api: false` (YouTube)
- [ ] X `rss_feed_url` 동작 확인 (브라우저/curl)
- [ ] `poll_minutes` ≥ 30
- [ ] `feedback.store_db: false`
- [ ] `REDIS_URL` / `DATABASE_URL` 미설정
- [ ] Discord Message Content Intent (모더레이션·FAQ 시)

### 배포 후

- [ ] `systemd` 또는 Docker `restart: unless-stopped`
- [ ] cursor 파일 `data/` 백업 (VM 재생성 시 중복 게시 방지)
- [ ] Oracle: 유휴 리소스 회수 방지 — [Always Free 가이드](https://docs.oracle.com/iaas/Content/FreeTier/freetier_topic-Always_Free_Resources.htm) 준수

---

## 10. 무료 한도를 넘기기 쉬운 실수

| 실수 | 결과 |
|------|------|
| Render 무료에 배포 | 슬립 → 봇 오프라인, 모더레이션 무용 |
| X API 키 연동 | 유료 Tier·quota |
| YouTube API만 사용 | quota 소진 |
| `poll_minutes: 5` + 공개 RSSHub | IP 밴·429 |
| Redis/Postgres 필수로 설계 |托管 DB 유료화 |
| 멀티 인스턴스 + in-memory rate limit | 스팸 방지 깨짐 |

---

## 11. 규모별 다음 단계 (여전히 저비용)

| 규모 | 변경 | 추가 비용 |
|------|------|-----------|
| 소 (~1k 멤버) | 현 구성 유지 | $0 |
| 중 | SQLite + `poll_minutes: 15` | $0 |
| 대 | 유료 VM 1대 ($4~6/월), Redis | 소액 |

---

## 12. 요약

**PelagornisBot Lite를 $0으로 돌리려면:**

1. **Lite 스펙만** 구현 ([SPEC.md](./SPEC.md))  
2. **Discord + RSS(X) + RSS(YouTube)** — 유료 API 0개  
3. **DB 없음** — JSON + Discord 채널  
4. **Oracle Always Free VM** 또는 집 PC에서 **상시 1프로세스**  
5. **30분 폴링** + (권장) **같은 VM에 RSSHub**

이 구성이 이 레포의 **기본 운영 프로필**이다.
