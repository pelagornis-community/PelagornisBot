# PelagornisBot Lite

**문서 버전:** 1.3  
**작성일:** 2026-05-29  
**상태:** 확정 (MVP 구현 기준)  
**변경 (v1.3):** 프로젝트 = Lite only · 무료 운영 프로필 [FREE_TIER.md](./FREE_TIER.md)

---

## 1. 개요

### 1.1 목적

**외부(퍼블릭) 커뮤니티**를 가볍게 운영하기 위한 Discord 중심 봇이다.  
수동 반복 작업을 줄이고, 채널을 깨끗하게 유지하며, **X·YouTube 활동을 커뮤니티에 자동 노출**한다.

| 목표 | 설명 |
|------|------|
| 커뮤니티 활성 유지 | 신규 유입 환영, 소셜 콘텐츠 자동 공유 |
| 운영 부담 감소 | 모더레이션·FAQ·홍보 자동화 |
| 환경 정리 | 스팸·금지어·신규 유저 링크 제한 |

### 1.2 설계 원칙

1. **Keep it simple** — 모듈은 독립적이지만 배포는 단일 프로세스
2. **Automate repetitive tasks** — 홍보·규칙 안내·FAQ는 사람 손 최소화
3. **Visibility first** — 활동 피드(트윗·영상·공지)가 채널에 보이게
4. **Easy to maintain** — 설정은 YAML/환경 변수, DB는 선택
5. **Free-tier first** — 유료 API·DB 없이 $0 운영 가능 ([FREE_TIER.md](./FREE_TIER.md))

### 1.3 1차 플랫폼

| 플랫폼 | MVP | 비고 |
|--------|:---:|:--|
| **Discord** | ✓ | **유일한 봇 플랫폼** |
| X (Twitter) | ✓ | **RSS 피드** 폴링 (공식 API·Bearer Token 사용 안 함) |
| YouTube | ✓ | **Atom RSS** 권장 (§3.3.2, 무료) |

### 1.4 비목표 (이 프로젝트에서 하지 않음)

- **Slack** 및 그 밖의 메신저 어댑터
- 내부용 CMS·승인 워크플로·멀티 테넌트 SaaS
- AI 모더레이션·감정 분석
- 티켓 시스템·결제·역할 스토어
- X API v2 / 유료 Developer Tier
- PostgreSQL / Redis (**MVP·무료 프로필** — §1.6)

### 1.6 무료($0) 운영 프로필 (기본)

| 항목 | 기본값 |
|------|--------|
| 호스팅 | Oracle Always Free VM 또는 집 PC (24/7) |
| YouTube | `use_api: false` + RSS URL |
| X | RSS only |
| DB | 없음 — `data/*.json` + Discord 채널 |
| 폴링 | `poll_minutes: 30` |
| 설정 | [config/profiles/free-tier.yaml](../config/profiles/free-tier.yaml) |

상세: [FREE_TIER.md](./FREE_TIER.md)

---

## 2. 모듈 구조

```typescript
interface Bot {
  onboarding: OnboardingModule;
  moderation: ModerationModule;
  autoPromotion: PromotionModule;
  announcement: AnnouncementModule;
  faq: FAQModule;
  feedback: FeedbackModule;
  admin: AdminModule;
  security?: SecurityModule;  // Optional, 권장
}
```

각 모듈은 **이벤트 버스** 또는 **미들웨어 체인**으로 연결한다.

```
Discord Event
    → security (optional, 선처리)
    → moderation
    → faq (자동 응답, 삭제 안 함)
    → onboarding (join / first message)
    → handlers...
```

---

## 3. 기능 상세

### 3.1 Onboarding & Rules

| 기능 | 트리거 | 동작 |
|------|--------|------|
| 환영 메시지 | `guildMemberAdd` | 설정 채널 또는 DM에 환영 + `/rules` 안내 |
| 규칙 조회 | `/rules` | 규칙 Embed 목록 (설정 파일 또는 DB) |
| 첫 메시지 리마인더 | `messageCreate` (유저당 1회) | “규칙 확인: `/rules`” (설정 채널만 또는 전역) |

**환영 메시지 예시**

```
환영합니다, {user}! 커뮤니티 규칙을 확인해 주세요: /rules
```

**설정**

```yaml
onboarding:
  welcome_channel_id: "123..."
  welcome_dm: false
  welcome_template: "환영합니다, {user}! 규칙: /rules"
  rules:
    - title: "존중"
      body: "비방·혐오 발언 금지"
    - title: "스팸"
      body: "무단 홍보·도배 금지"
  first_message_reminder: true
  first_message_template: "첫 메시지 감사합니다. 규칙: /rules"
```

---

### 3.2 Basic Moderation

| 기능 | 설명 | 기본 동작 |
|------|------|-----------|
| 금지어 필터 | 단어·패턴 목록 | 메시지 삭제 + (선택) 경고 DM |
| 스팸 방지 | N초 내 M개 메시지 | 타임아웃(mute) 또는 역할 제한 |
| 신규 유저 링크 제한 | 가입 후 X시간 미만 | URL 포함 메시지 삭제 + 안내 |

**의사 코드**

```pseudo
on messageCreate(msg):
  if security.maskAndAlert(msg): return  # API 키 등 (§3.8)

  if moderation.containsBannedWord(msg.content):
    delete(msg)
    log("banned_word", user, msg.id)
    return

  if user.isNew() and moderation.containsLink(msg.content):
    delete(msg)
    reply ephemeral "가입 후 {hours}시간 이후 링크 공유 가능합니다"
    return

  if moderation.rateLimitExceeded(user.id):
    timeout(user, duration=10m)
    log("spam", user)
    return
```

**설정**

```yaml
moderation:
  banned_words: ["word1", "word2"]
  banned_patterns: ["discord\\.gg\\/\\w+"]  # regex, optional
  rate_limit:
    window_seconds: 10
    max_messages: 5
    action: timeout  # warn | timeout | kick
    timeout_minutes: 10
  new_user:
    link_restrict_hours: 24
    exempt_role_ids: ["verified_role_id"]
  log_channel_id: "mod-log-channel"
```

**권한:** 봇에 `Manage Messages`, `Moderate Members`, `Read Message History` 필요.

---

### 3.3 Auto Promotion (Core)

X·YouTube 새 콘텐츠를 **하나의 Discord 홍보 채널**(`promotion.discord_channel_id`)에 자동 게시한다.

#### 3.3.0 공통 — 단일 채널

| 항목 | 결정 |
|------|------|
| Discord 채널 | **1개** (예: `#updates`, `#social-feed`) |
| X / YouTube | 동일 채널에 Embed 게시, `source` 필드로 구분 |
| 폴링 | 소스별 cron 가능, 기본 `poll_minutes` 공통 |

```pseudo
PROMO_CHANNEL = config.promotion.discord_channel_id

every poll_interval:
  for source in [x_rss, youtube]:
    items = source.fetchNew(since_cursor=last[source])
    for item in items sorted by publishedAt asc:
      post_to_discord(
        channel=PROMO_CHANNEL,
        title=item.badge + item.title,   # "📢 …" or "🎬 …"
        url=item.url,
        description=item.description,
        footer=source.name
      )
      last[source] = item.id
```

#### 3.3.1 X (Twitter) — RSS

X는 공식 API를 쓰지 않고 **RSS/Atom 피드 URL**을 주기적으로 읽는다.

| 항목 | 내용 |
|------|------|
| 감지 방식 | HTTP GET → XML 파싱 (`rss-parser` 등) |
| 피드 URL | 설정에 `rss_feed_url` (운영자가 준비) |
| 인증 | **없음** (공개 피드 URL) |
| 중복 방지 | RSS `guid` 또는 `link`를 cursor로 저장 |
| 주기 | 기본 10~15분 (피드 제공자 rate limit 준수) |

**피드 URL 준비 (운영자 작업)**

X는 네이티브 RSS를 제공하지 않는다. 아래 중 하나로 **트윗 타임라인 RSS**를 만든 뒤 URL을 config에 넣는다.

| 방식 | 예시 | 비고 |
|------|------|------|
| RSSHub (권장) | `https://rsshub.app/twitter/user/{username}` | 자체 호스트 시 안정적 |
| rss.app 등 | 서비스에서 생성한 feed URL | 유료·제한 확인 |
| 기타 브리지 | Nitter 계열 | 가용성 낮음, 비권장 |

```pseudo
every poll_interval:
  feed = fetch_rss(config.promotion.x.rss_feed_url)
  for entry in feed.items where entry.id > last_x_cursor:
    post_to_discord(
      channel=PROMO_CHANNEL,
      title="📢 New Tweet",
      url=entry.link,
      description=entry.contentSnippet
    )
    last_x_cursor = entry.id
```

#### 3.3.2 YouTube

| 항목 | 내용 |
|------|------|
| 감지 방식 (권장 Lite) | **Atom RSS** — API 키 불필요 |
| RSS URL | `https://www.youtube.com/feeds/videos.xml?channel_id={CHANNEL_ID}` |
| 감지 방식 (대안) | YouTube Data API `playlistItems` (quota 관리 필요) |
| 중복 방지 | `yt:videoId` 또는 RSS `id` |

```pseudo
every poll_interval:
  feed = fetch_rss(youtube_rss_url)   # 또는 API
  for entry in new_items(feed):
    post_to_discord(
      channel=PROMO_CHANNEL,
      title="🎬 New Video",
      url=entry.link,
      description=entry.title
    )
```

**Discord 포스트 포맷 (공통 Embed)**

```
제목: 📢 New Tweet / 🎬 New Video
설명: (RSS description / 영상 제목)
링크: [보기](url)
푸터: X · RSS / YouTube · 자동 게시
```

**설정 (확정)**

```yaml
promotion:
  enabled: true
  discord_channel_id: "PROMO_CHANNEL_ID"   # X + YouTube 공통
  poll_minutes: 15
  x:
    enabled: true
    rss_feed_url: "https://rsshub.example/twitter/user/your_handle"
  youtube:
    enabled: true
    # Lite 권장: RSS (API 키 없음)
    rss_feed_url: "https://www.youtube.com/feeds/videos.xml?channel_id=UCxxxxxxxx"
    # 대안: Data API
    use_api: false
    channel_id: "UCxxxxxxxx"
```

**Rate limit / 안정성:** RSS 제공자·YouTube에 무리한 간격(≥10분) 유지. 실패 시 exponential backoff + (선택) mod-log 알림.

---

### 3.4 Announcement & Subscription

| 기능 | 명령 | 권한 | 설명 |
|------|------|------|------|
| 공지 | `/announce` | Mod+ | 제목·본문·(선택) 멘션 → 지정 채널 게시 |
| 구독 | `/subscribe` | Everyone | promotion/announce 알림 역할 부여 |
| 구독 해제 | `/unsubscribe` | Everyone | 역할 제거 |
| 핀 | `/pin` (admin) | Mod+ | 봇 메시지 또는 메시지 ID 핀 |

**`/announce` 옵션**

- `title`, `message`, `channel` (optional, default 공지 채널)
- `pin`: boolean
- `mention`: none | here | role_id (Mod+만)

**구독 모델 (MVP)**

- Discord **역할** `@notify-promo` 부여/제거 (버튼 또는 slash)
- 공지 게시 시 해당 역할 mention (워크스페이스 정책에 따라 `@here` 대신 역할 권장)

```yaml
announcement:
  default_channel_id: "..."
  subscriber_role_id: "..."
  allow_mention_here: false
```

---

### 3.5 Simple FAQ

키워드(또는 정규식) → 자동 응답. **명령 없이** 일반 메시지에서 동작.

```pseudo
on messageCreate(msg):
  if msg.author.bot: return
  match = faq.findMatch(msg.content)
  if match:
    reply(match.response, allowed_mentions=none)
```

**설정 예**

```yaml
faq:
  enabled: true
  cooldown_seconds: 30      # 같은 채널 스팸 방지
  rules:
    - keywords: ["download", "다운로드"]
      response: "다운로드: https://example.com/download"
    - keywords: ["support", "지원"]
      response: "지원: https://example.com/support"
  channels_only: []           # 비우면 전체 허용
```

**주의:** 모더레이션 삭제 후 FAQ가 돌지 않도록 파이프라인 순서 고정 (§2).

---

### 3.6 Feedback Collection

| 기능 | 설명 |
|------|------|
| `/feedback` | 모달 또는 옵션으로 내용 수집 |
| 저장 | MVP: 전용 Discord 채널에 포맷 포스트 / Phase 1.5: SQLite |
| 익명 | `anonymous: true` 옵션 시 사용자 ID 마스킹 |

**Discord 출력 예**

```
📩 Feedback (anonymous)
내용: ...
메타: guild_id, timestamp (user_id는 anonymous 시 생략)
```

```yaml
feedback:
  channel_id: "feedback-inbox"
  allow_anonymous: true
  store_db: false           # true 시 feedback 테이블
```

---

### 3.7 Admin Tools

| 명령 | 권한 | 동작 |
|------|------|------|
| `/delete` | Mod+ | 메시지 ID로 삭제 (또는 reply 대상) |
| `/mute` | Mod+ | timeout 분 단위 |
| `/unmute` | Mod+ | timeout 해제 |
| `/pin` | Mod+ | 메시지 핀 |
| `/config show` | Admin | (선택) 비민감 설정 요약 |

모든 Admin 액션은 `mod-log` 채널 + (선택) DB 감사 로그.

---

### 3.8 Security (Optional, 권장)

민감 정보 유출 방지 — **삭제하지 않고 마스킹 후 경고** 또는 **삭제** 정책 선택.

| 패턴 | 예 |
|------|-----|
| OpenAI-style key | `sk-...` |
| GitHub PAT | `ghp_...` |
| 이메일 | `*@*.*` (설정 가능) |

```pseudo
if message.matches(sensitive_patterns):
  if policy == "mask":
    edit_or_reply(masked_content)
  else:
    delete(message)
  notify(mod_log_channel)
```

```yaml
security:
  enabled: true
  action: delete   # mask | delete
  patterns:
    - name: openai_key
      regex: "sk-[a-zA-Z0-9]{20,}"
    - name: email
      regex: "[\\w.-]+@[\\w.-]+\\.\\w+"
  notify_mod_channel: true
```

---

## 4. 권한·역할

| 역할 (Discord) | 기능 |
|----------------|------|
| `@everyone` | `/rules`, `/subscribe`, `/feedback`, FAQ 수신 |
| `Mod` | `/announce`, `/mute`, `/delete`, `/pin` |
| `Admin` | 설정 변경, promotion 계정 연동 |
| 봇 | 위 Mod 권한 + 메시지 관리 |

MVP: Discord Role ID를 `config.yaml`에 하드코딩.  
Phase 2: `/config` 명령으로 일부 핫 리로드.

---

## 5. 데이터·상태

### 5.1 MVP 저장소

| 데이터 | 저장 | 이유 |
|--------|------|------|
| `last_tweet_id`, `last_video_id` | JSON 파일 또는 SQLite | 중복 게시 방지 |
| 금지어·FAQ·규칙 | **YAML** (git 관리) | 단순·리뷰 가능 |
| Feedback | Discord 채널 (MVP) | DB 없이 시작 |
| Rate limit 카운터 | **메모리** (Redis optional) | 단일 인스턴스면 in-memory |

### 5.2 Phase 1.5 (SQLite)

```sql
-- promotion_cursor
platform TEXT,  -- x_rss | youtube_rss | youtube_api
feed_key TEXT,  -- rss url hash or channel_id
last_item_id TEXT, updated_at

-- feedback
id, guild_id, user_id nullable, body, anonymous, created_at

-- mod_log (optional)
id, action, actor_id, target_id, payload, created_at
```

---

## 6. 아키텍처

```
┌─────────────────────────────────────────────────┐
│              Discord Bot Process                 │
│  ┌──────────┐ ┌────────────┐ ┌────────────────┐ │
│  │ Commands │ │ Event Bus  │ │ Scheduler      │ │
│  │ (slash)  │ │            │ │ (node-cron)    │ │
│  └────┬─────┘ └─────┬──────┘ └───────┬────────┘ │
│       │             │                 │         │
│  ┌────▼─────────────▼─────────────────▼───────┐ │
│  │ Modules: onboarding | moderation | faq   │ │
│  │          announcement | feedback | admin  │ │
│  │          security?                         │ │
│  └────┬───────────────────────────────┬──────┘ │
│       │                               │         │
│  ┌────▼──────┐                 ┌──────▼──────┐  │
│  │ Promotion │                 │ Config      │  │
│  │ Worker    │                 │ YAML + env  │  │
│  └────┬──────┘                 └─────────────┘  │
└───────┼─────────────────────────────────────────┘
        │
   ┌────▼────┐     ┌──────────────┐
   │ X RSS   │     │ YouTube RSS  │
   │ feed    │     │ (or API)     │
   └─────────┘     └──────────────┘
```

### 6.1 기술 스택 (권장)

| 항목 | 선택 |
|------|------|
| Runtime | **Go 1.22+** |
| Discord | **discordgo** |
| Cron | `robfig/cron` |
| Config | `config.yaml` + Zod 검증 |
| HTTP | `undici` / native fetch |
| RSS | `rss-parser` |
| DB (optional) | better-sqlite3 |
| 배포 | Docker, Railway/Fly/VPS |

### 6.2 디렉터리 구조

```
pelagornis-bot/
├── config/
│   ├── config.example.yaml
│   ├── rules.yaml
│   ├── faq.yaml
│   └── moderation.yaml
├── src/
│   ├── index.ts
│   ├── bot.ts
│   ├── events/
│   ├── commands/
│   └── modules/
│       ├── onboarding/
│       ├── moderation/
│       ├── promotion/
│       │   ├── rss.provider.ts      # 공통 RSS 파서
│       │   ├── x-rss.provider.ts
│       │   └── youtube-rss.provider.ts
│       ├── announcement/
│       ├── faq/
│       ├── feedback/
│       ├── admin/
│       └── security/
├── data/                 # gitignore (cursors)
└── docs/
```

---

## 7. 슬래시 명령 목록 (MVP)

| 명령 | 설명 |
|------|------|
| `/rules` | 규칙 Embed |
| `/announce` | 공지 게시 |
| `/subscribe` | 알림 역할 |
| `/unsubscribe` | 알림 해제 |
| `/feedback` | 피드백 제출 |
| `/delete` | 메시지 삭제 (Mod) |
| `/mute` | 타임아웃 (Mod) |
| `/unmute` | 타임아웃 해제 (Mod) |
| `/pin` | 메시지 핀 (Mod) |

---

## 8. 환경 변수

```bash
DISCORD_BOT_TOKEN=
DISCORD_CLIENT_ID=
DISCORD_GUILD_ID=              # 개발 시 slash 즉시 등록

# Promotion (URLs는 config.yaml — env는 선택 override)
# X_RSS_FEED_URL=
# YOUTUBE_RSS_FEED_URL=
YOUTUBE_API_KEY=              # promotion.youtube.use_api=true 일 때만

# Optional
DATABASE_PATH=./data/bot.sqlite
REDIS_URL=                     # rate limit / multi-instance
```

---

## 9. MVP 범위

### 9.1 Must Have

- [ ] Config YAML + Zod
- [ ] Onboarding (welcome + `/rules` + first message)
- [ ] Moderation (금지어, rate limit, 신규 링크)
- [ ] Promotion (X RSS + YouTube → **단일 채널**)
- [ ] `/announce`, `/subscribe`
- [ ] FAQ keyword
- [ ] `/feedback` → 채널
- [ ] Admin: `/delete`, `/mute`, `/pin`
- [ ] Security: API key 패턴 (optional flag)

### 9.2 Should Have (v1.1)

- [ ] SQLite feedback + promotion cursor
- [ ] Mod log Embed 통일
- [ ] `/config reload` (Admin)
- [ ] YouTube 썸네일 Embed

### 9.3 Could Have (v2)

- [ ] Twitch/Kick 연동
- [ ] X 공식 API (유료·필요 시만, 기본은 RSS 유지)
- [ ] 다국어 `/rules`

---

## 10. 일정 (가이드, 1인 4~6주)

| 주 | 작업 |
|----|------|
| 1 | 프로젝트 셋업, config, `/rules`, welcome |
| 2 | moderation + security |
| 3 | promotion RSS (X + YouTube → 한 채널) |
| 4 | announce, subscribe, FAQ, feedback |
| 5 | admin commands, mod log, 테스트 |
| 6 | Docker, 문서, 프로덕션 배포 |

---

## 11. 리스크

| 리스크 | 완화 |
|--------|------|
| RSS 브리지 장애 | RSSHub 자체 호스트, 30분 poll |
| YouTube | RSS 사용 시 quota 없음 |
| FAQ 오탐 | 채널 제한·cooldown |
| 단일 인스턴스 | in-memory rate limit (Redis는 유료 확장 시) |

---

## 🇰🇷 요약

- 규칙 안내 (환영 + `/rules` + 첫 메시지)
- 최소 관리 (금지어·스팸·신규 링크)
- 자동 홍보 (X RSS + YouTube → **Discord 한 채널**)
- **Discord only**
- 간단 FAQ + 피드백 + Admin 명령
- (권장) API 키·이메일 마스킹

**가볍지만 실제 운영 가능한 구조.**

---

*변경 시 문서 버전을 올리고 README의 문서 표를 갱신한다.*
