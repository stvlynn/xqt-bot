# Documentation

This directory documents how xqt-bot is built, organized, and operated.
Keep it in sync with the code: if a change alters behavior, architecture,
configuration, deployment, or testing, update the relevant doc in the same
change set.

## Map

- [`project/`](project/README.md) — what the bot is, goals/non-goals.
  - [`project/architecture.md`](project/architecture.md) — layers, request/cron flows, KV key schema, dual compile targets.
- [`backend/`](backend/README.md) — DDD layered conventions with this project's real examples.
  - [`backend/domain.md`](backend/domain.md) — entities, aggregates, ports.
  - [`backend/application.md`](backend/application.md) — use-case services, sentinel errors.
  - [`backend/infrastructure.md`](backend/infrastructure.md) — KV, Telegram, LLM, image, config adapters.
  - [`backend/interfaces.md`](backend/interfaces.md) — bot handlers, callback protocol, HTTP mux, cron.
- [`operations/`](operations/README.md) — run and ship the bot.
  - [`operations/local-dev.md`](operations/local-dev.md) — host mode and `wrangler dev`.
  - [`operations/deployment.md`](operations/deployment.md) — kv-setup → secrets → deploy → webhook.
- [`quality/`](quality/README.md) — quality gates.
  - [`quality/testing.md`](quality/testing.md) — testing strategy per layer.
  - [`quality/code-review.md`](quality/code-review.md) — review checklist.
- [`decisions/`](decisions/README.md) — architecture decision records (ADRs).

## How to use this documentation

1. New to the project: read [`project/architecture.md`](project/architecture.md) first.
2. Before writing code in a layer, read the matching `backend/` doc.
3. Before changing build/deploy behavior, read [`operations/deployment.md`](operations/deployment.md).
4. Significant trade-offs get an ADR under [`decisions/`](decisions/README.md).
