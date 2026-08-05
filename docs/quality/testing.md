# Testing

Everything runs on the host target with `go test ./...` (part of
`make check`). There are no browser/e2e tests; confidence comes from the
layer boundaries: every external dependency is a `ports` interface, so each
layer is tested behind its own seam.

## Per-layer strategy

### `internal/domain` — pure unit tests

Domain types have no dependencies, so tests exercise rules directly:
filter-rule matching (word/regex, case-insensitivity), captcha challenge
generation and expiry, ring-buffer eviction, task `Due`/`Rescheduled`,
payload encode/parse round-trips.

### `internal/application` — services against fake ports

`fakes_test.go` (same package, white-box) implements every `ports` interface
in memory — settings/captcha/msglog/activity/task repositories, telegram and
LLM gateways — and records calls for assertions. Tests inject a fixed clock
(`fixedNow`) so time-dependent behavior (expiry sweeps, due tasks, zombie
thresholds) is deterministic. Example pattern: build a service with fakes,
run the use case, assert on recorded gateway calls and stored state.

### `internal/infrastructure` — pure logic + `httptest`

- **kv**: every repository is tested against `MemoryStore`, which shares the
  `Store` contract with the production `cfStore` — key formats, JSON
  round-trips, TTL computation, and sweep behavior (`ListExpired`,
  corrupt-entry skipping) are all covered without Cloudflare.
- **llm**: the gateway is tested against an `httptest.Server` standing in
  for the OpenAI-compatible endpoint (request shape, `NONE` handling, error
  mapping). The unconfigured gateway (`Available() == false`) fails fast and
  is tested too.
- **config**: `load` takes a getter function, so tests supply a fake env map.

### `internal/interfaces` — pure parsing + mux tests

- `bot/parse_test.go` table-tests the pure helpers: command parsing,
  `@otherbot` targeting, the `c:`/`m:` callback-data codecs, pattern/emoji
  argument splitting.
- `http/mux_test.go` exercises the webhook mux over `httptest`: wrong/missing
  secret header → 401, malformed JSON → 400, valid update → always 200 even
  when the handler errors.

### Not tested (by design)

- `telegram.Gateway` is a thin pass-through to `go-telegram/bot`; its logic
  (keyboard building) is pure and the rest is SDK calls.
- `store_js.go`, `main_js.go` and other js/wasm-only shims are verified by
  the wasm build in `make check`, not by unit tests.

## Conventions

- Tests are white-box (`package foo`) where clocks or internals need
  injection, black-box otherwise.
- No network, no wall-clock dependence, no shared mutable state between
  tests.
- Name tests after behavior (`TestSolve_ExpiredSessionKicked`), table-drive
  parsers and matchers.
