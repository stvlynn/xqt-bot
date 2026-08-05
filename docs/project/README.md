# Project

xqt-bot is a Telegram group-management bot: pure Go, compiled to WebAssembly,
deployed on Cloudflare Workers (webhook + Cron Trigger), state in Cloudflare KV.

## Documents

- [`architecture.md`](architecture.md) — layers, module list, request/cron
  flows, KV key schema, dual compile targets.

## What the bot does

- One-time, expiring invite links handed out through `/start` deep links.
- Group moderation: remotely imported word lists (`/filter import`, refreshed
  daily) + custom word/regex rules, join captcha (button or image),
  reply-to-moderate `/kick` `/ban` `/mute`.
- Chat summaries through any OpenAI-compatible LLM (`/summary`, optional
  recurring auto-summary).
- Scheduled tasks driven by a 5-minute Cron Trigger: auto summaries, zombie
  (inactive member) cleanup, expired-captcha sweeps.
- Auto emoji reactions (keyword/regex rules or LLM-picked) and small fun
  commands (`/roll`, `/pick`, welcome messages).

## Non-goals

- No public HTTP API — the only HTTP surface is the Telegram webhook plus
  `/healthz`. There is no frontend, no REST versioning, no database server.
- No multi-bot or multi-tenant hosting; one worker serves one bot token.

## Boundaries

- Backend layering follows [`../backend/README.md`](../backend/README.md)
  (DDD). Telegram user-facing copy is Simplified Chinese and lives only in
  `internal/interfaces/bot/texts.go`; everything else (code, docs, commits)
  is English.
- Operations (local dev, deploy) live in [`../operations/`](../operations/README.md).
