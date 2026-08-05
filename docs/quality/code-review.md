# Code Review

## Checklist

### Architecture

- [ ] Does the change respect the DDD layer boundaries (`interfaces` → `application` → `domain`, `infrastructure` implements ports)?
- [ ] Is business logic in the domain layer, not in handlers or adapters?
- [ ] Are new external dependencies introduced as `domain/ports` interfaces and implemented in `infrastructure`?
- [ ] Does platform-specific code stay behind the `js && wasm` / `!js || !wasm` file pairs, keeping both targets compiling?

### Code quality

- [ ] Is all code and commentary in English, with user-facing copy only in `internal/interfaces/bot/texts.go`?
- [ ] Are there no hardcoded secrets or tokens?
- [ ] Is there no duplicated logic that could be extracted?
- [ ] Is there no fallback/clever bypass logic that masks a root cause?
- [ ] Do outbound HTTP calls use the shared client (`newHTTPClient`), not ad-hoc clients?

### Testing

- [ ] Are domain rules covered by unit tests?
- [ ] Are application services tested against the in-memory fakes?
- [ ] Do new parsers have table tests, and does `make check` pass?

### Documentation

- [ ] If behavior or conventions changed, is `docs/` updated in the same change set?
- [ ] If the KV schema, commands, or `wrangler.toml` changed, are `architecture.md` / README updated?

### Security

- [ ] Is user input validated at the boundary (handlers, `parse.go`)?
- [ ] Are secrets and message contents kept out of logs?
- [ ] Does the webhook still always answer 200 (no Telegram retry storms)?

## Review culture

- Reviews should be constructive and specific.
- Prefer asking questions over making demands.
- Approve only when the checklist is satisfied.
