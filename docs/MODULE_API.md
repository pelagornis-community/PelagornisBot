# Module API — PelagornisBot Lite

> **구현 언어:** Go (`internal/`). 아래 TypeScript는 논리 구조 참고용.

[SPEC.md](./SPEC.md) · [DECISIONS.md](./DECISIONS.md) · [FREE_TIER.md](./FREE_TIER.md)

---

## 1. Core Types

```typescript
import type { Client, Interaction, Message, GuildMember } from "discord.js";
import type { z } from "zod";
import type { AppConfig } from "./config.schema";

export interface BotContext {
  client: Client;
  config: AppConfig;
  logger: Logger;
  store: StateStore; // cursors, first-message flags
}

export interface BotModule {
  readonly name: string;
  register(ctx: BotContext): Promise<void> | void;
}
```

---

## 2. State Store

```typescript
export interface StateStore {
  getPromotionCursor(platform: "x_rss" | "youtube_rss" | "youtube_api", feedKey: string): Promise<string | null>;
  setPromotionCursor(platform: "x_rss" | "youtube_rss" | "youtube_api", feedKey: string, itemId: string): Promise<void>;

  hasSeenFirstMessage(userId: string, guildId: string): Promise<boolean>;
  markFirstMessage(userId: string, guildId: string): Promise<void>;
}

// MVP: JsonStateStore (data/cursors.json, data/first_message.json)
// v1.1: SqliteStateStore
```

---

## 3. Module Interfaces

### 3.1 OnboardingModule

```typescript
export interface OnboardingModule extends BotModule {
  name: "onboarding";
  // Events: guildMemberAdd
  // Commands: rules
}
```

| Hook | When |
|------|------|
| `onMemberJoin(member: GuildMember)` | Welcome |
| `onFirstMessage(message: Message)` | Rule reminder (once) |

---

### 3.2 ModerationModule

```typescript
export interface ModerationModule extends BotModule {
  name: "moderation";
  // Events: messageCreate (before FAQ)
}

export interface ModerationResult {
  action: "none" | "delete" | "timeout";
  reason?: string;
  timeoutMinutes?: number;
}

export interface ModerationService {
  check(message: Message): Promise<ModerationResult>;
}
```

---

### 3.3 PromotionModule

```typescript
export interface PromotionModule extends BotModule {
  name: "promotion";
  // Scheduler: cron
}

export interface PromotionProvider {
  readonly platform: "x_rss" | "youtube_rss" | "youtube_api";
  readonly feedKey: string; // rss_feed_url hash or channel_id
  fetchNewItems(sinceId: string | null): Promise<PromotionItem[]>;
}

/** 공통 RSS 구현 — X·YouTube Atom 모두 재사용 */
export interface RssPromotionProvider extends PromotionProvider {
  readonly rssFeedUrl: string;
}

export interface PromotionItem {
  id: string;
  title: string;
  url: string;
  description?: string;
  thumbnailUrl?: string;
  publishedAt: Date;
}
```

```typescript
// promotion.service.ts
async function runPoll(
  provider: PromotionProvider,
  channelId: string, // config.promotion.discord_channel_id — X·YT 공통
): Promise<void> {
  const since = await store.getPromotionCursor(provider.platform, provider.feedKey);
  const items = await provider.fetchNewItems(since);
  for (const item of items.sort(byPublishedAsc)) {
    await discord.postPromotionEmbed(channelId, item);
    await store.setPromotionCursor(provider.platform, provider.feedKey, item.id);
  }
}
```

---

### 3.4 AnnouncementModule

```typescript
export interface AnnouncementModule extends BotModule {
  name: "announcement";
  // Commands: announce, subscribe, unsubscribe
}
```

---

### 3.5 FAQModule

```typescript
export interface FAQModule extends BotModule {
  name: "faq";
}

export interface FAQRule {
  keywords: string[];
  response: string;
  caseSensitive?: boolean;
}

export interface FAQService {
  match(content: string): FAQRule | null;
}
```

실행 순서: `ModerationModule` 통과 후 `FAQModule`만 처리.

---

### 3.6 FeedbackModule

```typescript
export interface FeedbackModule extends BotModule {
  name: "feedback";
  // Commands: feedback (Modal)
}

export interface FeedbackPayload {
  body: string;
  anonymous: boolean;
  userId?: string;
  guildId: string;
}
```

---

### 3.7 AdminModule

```typescript
export interface AdminModule extends BotModule {
  name: "admin";
  // Commands: delete, mute, unmute, pin
}

export function requireMod(interaction: Interaction, modRoleIds: string[]): boolean;
```

---

### 3.8 SecurityModule (optional)

```typescript
export interface SecurityModule extends BotModule {
  name: "security";
}

export interface SecurityService {
  scan(content: string): { matched: boolean; masked?: string; patternName?: string };
}
```

파이프라인 **최상단**: `messageCreate` → security → moderation → faq.

---

## 4. Message Pipeline

```typescript
async function onMessageCreate(message: Message, ctx: BotContext) {
  if (message.author.bot || !message.guild) return;

  const sec = ctx.modules.security?.scan(message.content);
  if (sec?.matched) { /* delete or mask */ return; }

  const mod = await ctx.modules.moderation.check(message);
  if (mod.action !== "none") { await applyModeration(message, mod); return; }

  await ctx.modules.faq.tryReply(message);
  await ctx.modules.onboarding.tryFirstMessageReminder(message);
}
```

---

## 5. Config Schema (Zod sketch)

```typescript
const AppConfigSchema = z.object({
  guild_id: z.string(),
  onboarding: z.object({ /* ... */ }),
  moderation: z.object({ /* ... */ }),
  promotion: z.object({
    enabled: z.boolean(),
    discord_channel_id: z.string(),
    poll_minutes: z.number(),
    x: z.object({ enabled: z.boolean(), rss_feed_url: z.string().url() }).optional(),
    youtube: z.object({
      enabled: z.boolean(),
      rss_feed_url: z.string().url().optional(),
      use_api: z.boolean().optional(),
      channel_id: z.string().optional(),
    }).optional(),
  }),
  announcement: z.object({ /* ... */ }),
  faq: z.object({ enabled: z.boolean(), rules: z.array(z.object({ keywords: z.array(z.string()), response: z.string() })) }),
  feedback: z.object({ channel_id: z.string(), allow_anonymous: z.boolean() }),
  security: z.object({ enabled: z.boolean(), action: z.enum(["mask", "delete"]), patterns: z.array(z.object({ name: z.string(), regex: z.string() })) }).optional(),
  roles: z.object({ mod: z.array(z.string()), admin: z.array(z.string()) }),
});
```

전체 필드는 `config/config.example.yaml`과 동기화.

---

## 6. Bot Assembly

```typescript
const modules: BotModule[] = [
  securityModule,      // optional
  onboardingModule,
  moderationModule,
  faqModule,
  announcementModule,
  feedbackModule,
  adminModule,
  promotionModule,     // registers cron, no message hook
];

for (const m of modules) await m.register(ctx);
```

---

## 7. Extension Points (v2)

| 확장 | 방법 |
|------|------|
| Twitch | `RssPromotionProvider` 또는 전용 provider |
| Redis rate limit | `ModerationService` 구현체 교체 |
| Web dashboard | `FeedbackPayload` → HTTP POST |

---

*코드 생성 시 본 파일과 스펙 diff가 나면 스펙을 우선하고 문서를 갱신한다.*
