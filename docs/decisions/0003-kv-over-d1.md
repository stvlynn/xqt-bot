# ADR 0003: Cloudflare KV instead of D1

## Status

Accepted

## Context

The bot needs persistent per-chat state: settings aggregates, pending
captcha sessions, a bounded message log for summaries, last-seen timestamps
for zombie cleanup, and recurring-task entries. Cloudflare offers two
first-party stores usable from a worker: KV (key-value, eventually
consistent) and D1 (SQLite, relational).

The workload is strictly key-addressed:

- every read/write is by a deterministic key (`settings:<chatID>`,
  `captcha:<chatID>:<userID>`, …);
- values are self-contained JSON documents mirroring the domain types;
- the only "queries" are prefix scans over `captcha:` and `task:` during
  the 5-minute cron sweep, at a scale of at most hundreds of keys.

There are no joins, no ad-hoc queries, no relational invariants.

## Decision

Use one Cloudflare KV namespace behind a minimal `Store` interface
(`Get`/`Put`/`Delete`/`ListKeys`), with all key formats and JSON mapping
owned by `internal/infrastructure/kv`. Expiry is modeled with stored
timestamps evaluated in Go (plus a storage TTL backstop on captcha
sessions), not with storage-level features.

## Consequences

- Zero schema/migration machinery; the JSON tags on domain types are the
  persistence format. Changing them is a data migration and must be treated
  as one.
- KV's eventual consistency is harmless here: a settings write followed by
  a read in the same update hits the same edge, and worst case a stale read
  delays a toggle by seconds.
- Free-tier KV covers the bot comfortably (per-request operations, 1 KiB+
  values well under the 25 MiB value limit).
- Prefix scans (`ListKeys`) do not scale to huge key counts; acceptable for
  a group bot, and the captcha TTL self-cleans the hot prefix. Revisit if
  the task count ever grows large.

## Alternatives considered

- **D1 (SQLite)** — strongly consistent and queryable, but buys relational
  features the bot never uses at the cost of migrations, a schema to
  maintain, and a heavier driver path from Go-wasm (SQL over JS bindings
  instead of a four-method KV client).
- **Durable Objects** — strongly consistent per-chat state and alarms, but
  a much bigger programming model for state that has no coordination
  requirements; the 5-minute cron sweep already covers scheduling.
- **R2 / plain Workers state** — object storage has no list-by-prefix read
  pattern suited to small mutable documents.

## References

- `internal/infrastructure/kv/` (Store, repositories), `wrangler.toml`
  `[[kv_namespaces]]`
- Key schema: [`../project/architecture.md`](../project/architecture.md#kv-key-schema)
