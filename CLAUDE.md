# xqt-bot — Agentic Coding Guidelines

> Telegram group-management bot. Pure Go backend, compiled to WebAssembly and
> deployed on Cloudflare Workers. Read this file first, then the doc map below.

---

## Before you start

1. **Read the docs first.** `docs/` documents the conventions so agents do not have to guess.
2. **No secrets in code.** Bot tokens, webhook secrets, and LLM API keys live in
   Cloudflare Worker secrets (`wrangler secret put`) or `.dev.vars` locally.
   `.dev.vars` is git-ignored and must never be committed.
3. **Keep it compiling for two targets.** `go build ./...` (host, for tests) and
   `GOOS=js GOARCH=wasm go build ./...` (Workers) must both pass. Any file using
   `syscall/js` or `github.com/syumai/workers/cloudflare` must live behind
   `//go:build js && wasm`, with a host fallback behind `!js || !wasm`.

---

## Documentation map

- [`docs/project/architecture.md`](docs/project/architecture.md) — module boundaries, data flow, KV key schema.
- [`docs/backend/README.md`](docs/backend/README.md) — DDD entry point.
- [`docs/backend/domain.md`](docs/backend/domain.md) — entities, value objects, ports.
- [`docs/backend/application.md`](docs/backend/application.md) — use cases and services.
- [`docs/backend/infrastructure.md`](docs/backend/infrastructure.md) — KV, Telegram, LLM, image adapters.
- [`docs/backend/interfaces.md`](docs/backend/interfaces.md) — bot handlers, HTTP, cron adapters.
- [`docs/operations/local-dev.md`](docs/operations/local-dev.md) — local setup, `.dev.vars`, webhook tunneling.
- [`docs/operations/deployment.md`](docs/operations/deployment.md) — wrangler deploy, secrets, KV provisioning.
- [`docs/quality/testing.md`](docs/quality/testing.md) — testing strategy.

---

## Language and quality rules

### English for code, Chinese for bot copy

- All source code, comments, commit messages, and docs are written in English.
- Telegram user-facing copy is **Simplified Chinese** and lives in exactly one
  place: `internal/interfaces/bot/texts.go`. Never hardcode user-facing strings
  elsewhere. Keep copy short and self-explanatory — a new group admin must
  understand every message without reading docs.

### Forbidden patterns

- **No hardcoded secrets or tokens**, ever.
- **No duplicated implementations.** Reuse or extract to the correct layer.
- **No fallback/clever bypass logic.** Do not mask a root cause with a silent
  catch or a default value. Face the actual problem.
- **No mock/MVP stubs.** Features are implemented end to end or not at all.

---

## Backend: Domain-Driven Design (DDD)

- **`internal/domain/`** — business rules and ports. No framework, database, or
  transport imports. Sub-packages: `chat`, `moderation`, `reaction`, `summary`,
  `schedule`, `invite`, `ports`.
- **`internal/application/`** — use-case services. Depends only on `domain`.
  All dependencies arrive as `ports` interfaces; services return structured
  results, never user-facing prose.
- **`internal/infrastructure/`** — `kv` (Cloudflare KV repositories),
  `telegram` (Bot API gateway), `llm` (OpenAI-compatible client), `image`
  (PNG renderer), `config` (env loading).
- **`internal/interfaces/`** — `bot` (update handlers, keyboards, texts),
  `http` (webhook endpoint), `cron` (scheduled runner).
- Dependencies point inward: `interfaces` → `application` → `domain`;
  `infrastructure` implements `domain/ports`.
- Keep handlers thin: parse the update, call an application service, format
  the reply from `texts.go`.

---

## Commit conventions

[Conventional Commits](https://www.conventionalcommits.org/):
`<type>(<scope>): <subject>`, e.g. `feat(moderation): add regex filter rules`.
Types: `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `build`, `ci`, `perf`.

---

## Self-evolution rule

When a change alters behavior, architecture, configuration, deployment, or
testing expectations, update the relevant doc under `docs/` in the same
change set.
