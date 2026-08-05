# Quality

Quality gates for xqt-bot. The single command that must pass before every
commit is `make check` = `go vet ./...` + `go test ./...` + wasm build
(`npm run build`).

## Documents

- [`testing.md`](testing.md) — testing strategy per layer.
- [`code-review.md`](code-review.md) — review checklist.

## Principles

- Tests are deterministic: clocks and RNG are injected, fakes are in-memory.
- Both compile targets stay green: `go build ./...` (host) and
  `GOOS=js GOARCH=wasm go build ./...` (worker). The wasm build in
  `make check` is also the size guard for the 3 MiB free-plan limit.
- Formatting is `gofmt` (`make fmt`); `make lint` runs `golangci-lint` when
  installed.

## Definition of done

- Behavior matches the docs; a change that alters behavior, architecture,
  configuration, deployment, or testing updates the relevant `docs/` file in
  the same change set.
- Domain and application logic is covered by unit tests; new parsing helpers
  in the interfaces layer get table tests.
- No hardcoded secrets, no user-facing copy outside `texts.go`, no
  duplicated implementations, no silent fallbacks that mask root causes.
