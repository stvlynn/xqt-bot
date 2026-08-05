# Architecture

> xqt-bot is a Telegram group-management bot written in Go, compiled to
> WebAssembly and deployed on Cloudflare Workers. This file documents the
> real system; update it when modules, flows, or the KV schema change.

## Overview

Single deployable unit: one Cloudflare Worker serving the Telegram webhook
and one Cron Trigger every 5 minutes. All state lives in one Cloudflare KV
namespace. The codebase is a strict DDD layering inside `internal/`:

```text
Telegram                    Cloudflare Workers
   │ POST /webhook               │
   ▼                             ▼
┌─────────────────────────────────────────────────────┐
│ interfaces   bot (update handlers, texts)            │
│              http (webhook mux)  cron (runner glue)  │
├─────────────────────────────────────────────────────┤
│ application  10 use-case services + TaskRunner       │
├─────────────────────────────────────────────────────┤
│ domain       entities, value objects, ports          │
├─────────────────────────────────────────────────────┤
│ infrastructure  kv · telegram · llm · image · config │
└─────────────────────────────────────────────────────┘
        │                    │                │
        ▼                    ▼                ▼
  Cloudflare KV      Telegram Bot API   OpenAI-compatible LLM
```

Dependencies point inward: `interfaces` → `application` → `domain`;
`infrastructure` implements the `domain/ports` interfaces. Wiring happens
once in `setup()` in `main.go`, which runs on both compile targets.

## Modules

| Module | Responsibility |
|--------|----------------|
| `internal/domain/chat` | `Settings` aggregate: all per-chat config |
| `internal/domain/moderation` | filter rules, captcha challenge/session |
| `internal/domain/reaction` | auto-reaction rules, emoji whitelist |
| `internal/domain/summary` | recorded message + bounded ring buffer |
| `internal/domain/schedule` | recurring task model (`auto_summary`, `zombie_clean`, `filter_refresh`) |
| `internal/domain/invite` | deep-link payload encoding (`j<chatID>`) |
| `internal/domain/ports` | repository + gateway interfaces, `ErrNotFound` |
| `internal/application` | Captcha, Settings, Moderation, Invite, Reaction, Summary, Zombie, Fun services, `GroupMessagePipeline`, `TaskRunner` |
| `internal/infrastructure/kv` | all repositories over a minimal `Store` interface |
| `internal/infrastructure/telegram` | `ports.TelegramGateway` via `go-telegram/bot` |
| `internal/infrastructure/llm` | `ports.LLMGateway` via any OpenAI-compatible endpoint |
| `internal/infrastructure/image` | captcha PNG renderer (`golang.org/x/image`) |
| `internal/infrastructure/wordlist` | `ports.WordListGateway` — remote filter-list fetch + parse |
| `internal/infrastructure/config` | env loading (per-platform) |
| `internal/interfaces/bot` | update routing, command parsing, keyboards, all user-facing copy (`texts.go`) |
| `internal/interfaces/http` | `POST /webhook`, `GET /healthz`, `GET /` |
| `internal/interfaces/cron` | adapts the scheduled event to `TaskRunner` |

## Webhook request flow

1. Telegram POSTs an update to `/webhook` with the header
   `X-Telegram-Bot-Api-Secret-Token` (registered by `scripts/setup-webhook.sh`).
2. `internal/interfaces/http/mux.go` verifies the header with a constant-time
   compare, decodes `models.Update`, and calls `bot.Handler.HandleUpdate`.
   The endpoint **always answers 200** — processing errors are logged, never
   reported, so Telegram does not retry and storm the worker.
3. `internal/interfaces/bot/handler.go` routes the update: new-member service
   messages → captcha/welcome flow; callback queries → captcha answer (`c:`)
   or admin panel (`m:`); commands → one `cmd*` method; ordinary group text →
   `GroupMessagePipeline` (activity touch → message log → moderation →
   auto-reaction, skipping reaction when moderation hit).
4. Handlers call application services, which enforce admin checks and
   business rules and talk to the outside world only through `ports`.
5. Replies go out through `ports.TelegramGateway` (Telegram Bot API) or
   `ports.LLMGateway` (summaries, AI reactions).

## Cron flow

`wrangler.toml` registers `crons = ["*/5 * * * *"]`. Per-chat intervals are
stored in KV, so the 5-minute tick is just the sweep granularity:

1. The scheduled event enters `main_js.go`, which wraps it via
   `cron.ScheduleTaskNonBlock` (non-blocking, shares the isolate).
2. `internal/interfaces/cron.RunOnce` calls `application.TaskRunner.Run`.
3. The runner sweeps expired captcha sessions (`captcha:` prefix, kicking
   members who never solved the challenge), then lists every `task:` key and
   executes due tasks: `auto_summary` (summarize + post), `zombie_clean`
   (kick inactive members) and `filter_refresh` (re-import the chat's remote
   word lists). Each executed task is rescheduled to
   `now + IntervalHours`. Individual failures are collected in the
   `RunReport`, never abort the sweep.

## KV key schema

One KV namespace (binding `KV`). All values are JSON. Repositories in
`internal/infrastructure/kv/` own the key formats:

| Key | Value | TTL | Notes |
|-----|-------|-----|-------|
| `settings:<chatID>` | `chat.Settings` aggregate | none | created with `chat.Default` on first contact |
| `captcha:<chatID>:<userID>` | `moderation.Session` (challenge + `ExpiresAt`) | challenge timeout + 60s grace | TTL self-cleans even if the sweeper never runs |
| `msglog:<chatID>` | `summary.Ring` (≤ 500 messages) | none | oldest evicted when full |
| `activity:<chatID>` | `map[userID]lastSeen` (user IDs as decimal string keys) | none | capped at 2000 entries, oldest-seen evicted; key deleted when empty |
| `task:<kind>:<chatID>` | `schedule.Task` (`NextRunAt`, `IntervalHours`) | none | `kind` is `auto_summary`, `zombie_clean` or `filter_refresh` |

Prefix scans (`ListKeys`) are used only by the cron sweeps (`captcha:`,
`task:`). Missing keys surface as `ports.ErrNotFound` (Cloudflare KV's
`<null>` sentinel is translated in `kv/store_js.go`).

## Two compile targets

Every platform-dependent file comes in a pair selected by build tags:

| js/wasm (`//go:build js && wasm`) | host (`!js \|\| !wasm`) | Difference |
|---|---|---|
| `main_js.go` | `main_host.go` | worker serve + cron vs. `http.ListenAndServe(":8787")` |
| `httpclient_js.go` | `httpclient_host.go` | Workers fetch transport vs. `http.Client{Timeout: 60s}` |
| `store_js.go` | `store_host.go` | Cloudflare KV binding vs. in-memory `MemoryStore` |
| `config/env_js.go` | `config/env_host.go` | `cloudflare.GetEnv` vs. `os.Getenv` |
| `kv/store_js.go` | — | the only KV client is js/wasm-only; host uses `MemoryStore` |

Both `go build ./...` and `GOOS=js GOARCH=wasm go build ./...` must pass
(enforced by `make check`). Anything importing `syscall/js` or
`github.com/syumai/workers/cloudflare` belongs behind `js && wasm`.

## Outbound HTTP on Workers

Go's default js/wasm HTTP transport calls the global `fetch` with the wrong
receiver and fails with `Illegal invocation` on the Workers runtime. All
outbound HTTP therefore goes through one client built in
`httpclient_js.go` from `syumai/workers/cloudflare/fetch`, injected into
both the Telegram bot (`tgbot.WithHTTPClient`) and the LLM gateway
(`llm.NewGatewayWithClient`). See
[`decisions/0002-workers-fetch-transport.md`](../decisions/0002-workers-fetch-transport.md).
The bot is also constructed with `tgbot.WithSkipGetMe()` because the wasm
module must not issue network requests during startup.
