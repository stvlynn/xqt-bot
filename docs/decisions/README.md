# Decisions

Architecture decision records (ADRs): why significant choices were made and
what alternatives were considered.

## When to write an ADR

Write an ADR when the decision:

- Is hard to reverse.
- Affects multiple layers or the deployment model.
- Has non-obvious trade-offs.

## Existing decisions

- [`0001-go-on-workers-via-wasm.md`](0001-go-on-workers-via-wasm.md) — Go → WebAssembly on Workers (`syumai/workers`), the 3 MiB free-plan budget and `-ldflags='-s -w'`.
- [`0002-workers-fetch-transport.md`](0002-workers-fetch-transport.md) — all outbound HTTP through the Workers fetch transport (`Illegal invocation` fix).
- [`0003-kv-over-d1.md`](0003-kv-over-d1.md) — Cloudflare KV as the only datastore.
- [`adr-template.md`](adr-template.md) — template for new ADRs.

## Naming

Use a four-digit sequential number and a short kebab-case title:

```
decisions/
  0001-go-on-workers-via-wasm.md
  0002-workers-fetch-transport.md
```
