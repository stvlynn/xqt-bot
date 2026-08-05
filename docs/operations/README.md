# Operations

How xqt-bot is run locally and shipped to Cloudflare Workers.

## Documents

- [`local-dev.md`](local-dev.md) — run the bot on your machine (host mode or `wrangler dev`).
- [`deployment.md`](deployment.md) — KV provisioning, secrets, deploy, webhook registration, `wrangler.toml` reference, GitHub-integrated deploys.

## Requirements

- Go 1.25+ (see `go.mod`).
- Node.js + `npm install -g wrangler` (npm is used only as a script runner;
  there are no JS dependencies).
- A Cloudflare account (`wrangler login`) and a Telegram bot token from
  @BotFather.

## Standard commands

```sh
make check         # vet + unit tests + wasm build (run before every commit)
make test          # go test ./...
make fmt           # gofmt -w .
go run .           # host mode: webhook on :8787 with in-memory store
make dev           # wrangler dev (needs .dev.vars)
make deploy        # build wasm bundle + wrangler deploy
```

## Secrets

Secrets never live in the repo. Locally they go in `.dev.vars`
(git-ignored, see `.dev.vars.example`); in production they are worker secrets
set via `make secrets` / `wrangler secret put`:

- `TELEGRAM_BOT_TOKEN` (required)
- `TELEGRAM_WEBHOOK_SECRET` (any long random string)
- `LLM_API_KEY` (optional; without it AI features report "not configured")

Non-secret config lives in `[vars]` in `wrangler.toml` (`LLM_BASE_URL`,
`LLM_MODEL`, `ENVIRONMENT`, `BOT_USERNAME`).
