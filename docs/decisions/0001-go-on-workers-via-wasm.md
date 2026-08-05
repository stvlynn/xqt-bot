# ADR 0001: Go on Cloudflare Workers via WebAssembly

## Status

Accepted

## Context

The bot needs a cheap, always-on host close to Telegram's webhook model:
no server to manage, per-request billing, and a free tier that covers a
group-management bot's traffic. Cloudflare Workers fits, but its native
languages are JavaScript/TypeScript and Rust — and Go (via WebAssembly)
through `github.com/syumai/workers`. The author's ecosystem and the
libraries we want (notably `github.com/go-telegram/bot` for the Bot API,
`golang.org/x/image` for captcha rendering) are Go.

The hard constraint is size: the Workers free plan limits a compressed
(gzip) worker to **3 MiB**, and a Go wasm binary starts much larger.

## Decision

Write the bot in pure Go, compile to WebAssembly with `syumai/workers`, and
deploy one worker with a Cloudflare KV namespace for state:

- `GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o build/app.wasm .` —
  stripping the symbol table and DWARF (`-s -w`) is what brings the bundle
  under the 3 MiB gzip limit. This flag is part of `npm run build`; removing
  it breaks deployment on the free plan.
- `go-telegram/bot` in webhook mode (`WithSkipGetMe`, no polling).
- Cloudflare KV as the only datastore (see ADR 0003).
- Platform differences isolated in `js && wasm` / `!js || !wasm` file pairs
  so the same code also builds and tests as a host binary.

## Consequences

- One language end to end; domain/application code is plain Go and unit
  tests run natively with `go test`.
- The wasm binary size is a standing budget: adding heavy dependencies can
  push the compressed bundle over 3 MiB, so `make check` includes the wasm
  build as a guard.
- Startup must not perform network I/O (the module initializes synchronously
  in the isolate), which is why the bot skips `getMe`.
- Debuggability is weaker than native Go: stack traces through wasm are
  noisier, so logs carry explicit context (chat ID, update ID).

## Alternatives considered

- **TypeScript worker** — first-class on Workers and smaller bundles, but
  gives up the Go libraries and type-checking discipline the project relies
  on, and duplicates effort in a second ecosystem.
- **Rust worker (workers-rs)** — smaller/faster wasm, but far more expensive
  to write and maintain for a CRUD-style bot, and the Telegram SDKs are
  less ergonomic than `go-telegram/bot`.
- **VPS / container running a Go binary with long polling** — simpler
  runtime, but requires operating a server and loses the free-tier,
  zero-ops model.

## References

- `package.json` build script; `main_js.go`, `main_host.go`
- https://github.com/syumai/workers
- https://developers.cloudflare.com/workers/platform/limits/#worker-size
