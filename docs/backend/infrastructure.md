# Infrastructure Layer

`internal/infrastructure/` implements `domain/ports`. Except for explicitly
platform-tagged files, every package is plain Go that compiles for both host
(tests, local runs) and js/wasm (the worker).

## `kv` — repositories over Cloudflare KV

All persistence goes through one minimal abstraction (`store.go`):

```go
type Store interface {
    Get(ctx, key) ([]byte, error)            // ports.ErrNotFound on miss
    Put(ctx, key, val []byte, ttlSeconds int) error
    Delete(ctx, key) error
    ListKeys(ctx, prefix) ([]string, error)  // sorted
}
```

- **Production**: `cfStore` (`store_js.go`, js/wasm only) binds the KV
  namespace from `wrangler.toml`. It translates Cloudflare's `<null>` miss
  sentinel into `ports.ErrNotFound` and follows the list cursor to completion.
- **Tests and host runs**: `MemoryStore`, an in-memory map. TTL is ignored —
  expiry logic in this project is driven by stored timestamps, not storage
  TTL (the captcha TTL is only a self-cleaning backstop).

Five repositories build keys and (de)serialize JSON on top of `Store`:
`SettingsRepository` (`settings:<chatID>`), `CaptchaRepository`
(`captcha:<chatID>:<userID>`, storage TTL = challenge timeout + 60s grace),
`MessageLogRepository` (`msglog:<chatID>`, 500-message ring),
`ActivityRepository` (`activity:<chatID>`, ≤ 2000 entries, user IDs as
decimal string keys), `TaskRepository` (`task:<kind>:<chatID>`). Full schema:
[`../project/architecture.md`](../project/architecture.md#kv-key-schema).

Repository conventions:

- Missing aggregate ≠ error where a zero value is sensible: message log and
  activity return empty containers on a miss; settings materialize
  `chat.Default` and persist it.
- Corrupt entries never abort sweeps: `List`/`ListExpired` skip
  undecodable values.
- IDs are part of the key, so values stay self-contained JSON documents.

## `telegram` — Bot API gateway

`Gateway` implements `ports.TelegramGateway` on `github.com/go-telegram/bot`:
sending/editing/deleting messages, inline keyboards (`buildMarkup` converts
`[][]ports.Button`), one-time invite links (`CreateChatInviteLink` with
`MemberLimit: 1`), restrict/ban/unban, reactions, and admin checks. The bot
instance is created in `main.go` with `WithSkipGetMe()` (no network at
startup) and an injected HTTP client (see below). Kick is modeled as
ban + unban; mute as `RestrictMember` until a timestamp.

## `llm` — OpenAI-compatible gateway

`Gateway` implements `ports.LLMGateway` against any chat-completions
endpoint (`LLM_BASE_URL` + `LLM_MODEL` + `LLM_API_KEY`; Cloudflare AI
Gateway works as a base URL). An empty API key yields `Available() == false`
and calls fail fast — AI features degrade to "not configured" instead of
breaking the bot. Requests are tuned by constants at the top of
`gateway.go` (temperature 0.3, max 800 tokens, 12k-rune input cap, 200-rune
per-message cap). Both prompts are Chinese and live in this file; summaries
instruct ≤ 300 chars, reaction picking answers one whitelisted emoji or
`NONE`.

## `image` — captcha renderer

`image.NewRenderer()` draws the arithmetic question as a PNG using
`golang.org/x/image` (pure Go, wasm-safe). Used only for
`/captcha image` mode; the bytes go out through
`TelegramGateway.SendPhoto`.

## `config` — environment loading

`config.Load()` reads the runtime config: `TELEGRAM_BOT_TOKEN` (the only
required value), `TELEGRAM_WEBHOOK_SECRET`, `BOT_USERNAME`, `KV_BINDING`
(default `KV`), `LLM_BASE_URL`/`LLM_MODEL`/`LLM_API_KEY`, `ENVIRONMENT`. The
env source is per-platform: `env_js.go` reads Cloudflare bindings/secrets,
`env_host.go` reads `os.Getenv`. The platform-independent core takes a
getter function so it is testable on the host.

## Outbound HTTP

Every outbound call (Telegram API, LLM API) shares the `*http.Client` built
by `newHTTPClient()` in `main.go`'s platform files: Workers fetch transport
on js/wasm, standard 60 s client on host. Never create ad-hoc clients in
adapters — see
[`../decisions/0002-workers-fetch-transport.md`](../decisions/0002-workers-fetch-transport.md).
