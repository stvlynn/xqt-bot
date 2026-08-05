# Backend

The backend is a single Go module (`github.com/stvlynn/xqt-bot`) organized
with Domain-Driven Design layering under `internal/`.

## Documents

- [`domain.md`](domain.md) — entities, aggregates, value objects, ports.
- [`application.md`](application.md) — use-case services, sentinel errors, pipeline and task runner.
- [`infrastructure.md`](infrastructure.md) — KV repositories, Telegram/LLM gateways, image renderer, config.
- [`interfaces.md`](interfaces.md) — bot update handlers, callback-data protocol, HTTP webhook mux, cron adapter.

> There is no SQL database, no REST API, and no structured-logging stack:
> state lives in Cloudflare KV (see the key schema in
> [`../project/architecture.md`](../project/architecture.md)), the only HTTP
> surface is the Telegram webhook, and logging is plain `log.Printf` to the
> worker console. Template docs for those topics were removed intentionally.

## Layer rules

```text
interfaces → application → domain ←(ports implemented by)─ infrastructure
```

- **`internal/domain/`** — business rules and ports. No framework, storage,
  or transport imports. Sub-packages: `chat`, `moderation`, `reaction`,
  `summary`, `schedule`, `invite`, `ports`.
- **`internal/application/`** — use-case services. Depends only on `domain`.
  All dependencies arrive as `ports` interfaces; services return structured
  results and sentinel errors, never user-facing prose.
- **`internal/infrastructure/`** — `kv`, `telegram`, `llm`, `image`,
  `config`. Implements `domain/ports`; compiles for both host and js/wasm
  unless a file is platform-specific.
- **`internal/interfaces/`** — `bot`, `http`, `cron`. Parses input, calls
  application services, renders replies from `texts.go`.

## Quick start

1. Read [`domain.md`](domain.md) to see where rules live (`chat.Settings`,
   `moderation.FilterRule`).
2. Read [`application.md`](application.md) before adding a use case.
3. Read [`interfaces.md`](interfaces.md) before adding a command or callback.
4. Keep handlers thin: parse → call service → format reply from `texts.go`.
