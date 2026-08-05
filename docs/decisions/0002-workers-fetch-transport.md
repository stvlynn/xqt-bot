# ADR 0002: Workers fetch transport for all outbound HTTP

## Status

Accepted

## Context

The worker makes outbound HTTP calls to the Telegram Bot API and to the
OpenAI-compatible LLM endpoint. Go's default `net/http` transport on
js/wasm calls the browser-style global `fetch` with an implicit
receiver. On the Cloudflare Workers runtime this fails at the first request
with `TypeError: Illegal invocation` — `fetch` in Workers must be invoked
with the correct receiver. Every outbound call from the wasm module was
broken out of the box.

## Decision

Build one `*http.Client` per entrypoint whose transport comes from
`syumai/workers/cloudflare/fetch` (`httpclient_js.go`):

```go
fetch.NewClient().HTTPClient(fetch.RedirectModeFollow)
```

and inject it everywhere outbound HTTP happens:

- the Telegram bot: `tgbot.New(token, tgbot.WithHTTPClient(0, httpClient))`
- the LLM gateway: `llm.NewGatewayWithClient(..., httpClient)`

On the host target the same factory returns a plain
`http.Client{Timeout: 60s}` (`httpclient_host.go`), so adapters never know
the difference. Rule: adapters accept an `*http.Client`; they never
construct their own.

## Consequences

- Telegram and LLM calls work identically on both targets; tests use
  `httptest` servers through the same code path.
- Any future outbound adapter must take the injected client — constructing
  a default client in js/wasm code reintroduces the `Illegal invocation`
  failure at runtime, where unit tests cannot catch it.

## Alternatives considered

- **Patch around Go's transport per call site** — scattered workarounds in
  each adapter; rejected as duplicated, easy-to-miss cleverness.
- **Use the platform's JS bindings directly for HTTP** (bypass `net/http`)
  — would force Telegram/LLM clients off their idiomatic Go APIs and split
  request logic across languages.

## References

- `httpclient_js.go`, `httpclient_host.go`, `main.go` (`setup`)
- https://pkg.go.dev/github.com/syumai/workers/cloudflare/fetch
