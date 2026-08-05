# Deployment

The bot deploys as a single Cloudflare Worker (Go → WebAssembly) with one KV
namespace and a 5-minute Cron Trigger.

## First-time setup

```sh
wrangler login

make kv-setup        # 1. create KV namespaces, patch ids into wrangler.toml
make secrets         # 2. interactively set worker secrets
make deploy          # 3. build wasm bundle + wrangler deploy
make webhook-setup   # 4. register webhook + command menu with Telegram
```

1. **`make kv-setup`** (`scripts/kv-setup.sh`) — runs
   `wrangler kv namespace create KV` and `... --preview`, then rewrites the
   `REPLACE_WITH_KV_ID` / `REPLACE_WITH_KV_PREVIEW_ID` placeholders in
   `wrangler.toml`. Commit the patched ids; they are not secrets.
2. **`make secrets`** (`scripts/secrets-setup.sh`) — prompts for
   `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, and (optionally)
   `LLM_API_KEY`, piping each to `wrangler secret put`. Empty input skips a
   secret; values never touch the repo.
3. **`make deploy`** — `npm run build` then `wrangler deploy`. The build is
   `workers-assets-gen -mode=go` (generates `build/worker.mjs` glue) plus
   `GOOS=js GOARCH=wasm go build -ldflags='-s -w' -o ./build/app.wasm .`.
   The `-s -w` flags strip debug info to stay inside the free plan's 3 MiB
   compressed-worker limit (see
   [`../decisions/0001-go-on-workers-via-wasm.md`](../decisions/0001-go-on-workers-via-wasm.md)).
4. **`make webhook-setup`** (`scripts/setup-webhook.sh`) — needs
   `TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, and `WORKER_URL`
   (e.g. `https://xqt-bot.<subdomain>.workers.dev`) in the environment. It
   calls `setWebhook` for `<WORKER_URL>/webhook` with the secret token and
   `allowed_updates: ["message", "callback_query", "my_chat_member"]`,
   registers the command menu via `setMyCommands`, and prints
   `getWebhookInfo` for verification.

Verify with `curl https://<worker>/healthz` (→ `ok`), then add the bot to a
group, promote it to admin, and send `/xqt`.

## Deploying from the Cloudflare Dashboard (GitHub integration)

Workers can build and deploy straight from this repository:

1. Workers → Create → Import repository → select `stvlynn/xqt-bot`.
2. Build command: `npm run build`. Entry point / deploy command stays the
  default (`wrangler deploy` with `main = ./build/worker.mjs`).
3. Add the KV binding named `KV` pointing at the same namespace id as in
   `wrangler.toml`, and set the three secrets in the Dashboard.
4. `[vars]` and the cron trigger are read from `wrangler.toml` automatically.

## `wrangler.toml` reference

| Field | Meaning |
|-------|---------|
| `name` | worker name; also the `<name>.workers.dev` subdomain |
| `main` | JS entry — the generated wasm loader (`build/worker.mjs`), not the Go source |
| `compatibility_date` | Workers runtime behavior pin |
| `workers_dev` | serve on the `workers.dev` subdomain (no custom route) |
| `[build].command` | what `wrangler deploy` / Dashboard builds run: `npm run build` |
| `[triggers].crons` | `*/5 * * * *` — sweep granularity only; per-chat intervals live in KV `task:` entries |
| `[[kv_namespaces]]` | binding `KV`; `id` = production namespace, `preview_id` = what `wrangler dev` uses |
| `[vars]` | non-secret config: `LLM_BASE_URL`, `LLM_MODEL`, `ENVIRONMENT`, `BOT_USERNAME` |

Secrets (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, `LLM_API_KEY`) are
intentionally absent — they are worker secrets, not file entries.

## Rollback

Deployments are immutable versions: `wrangler rollback` (or redeploy the
previous git commit) returns to the last good build. KV data is unaffected
by rollbacks; the JSON value formats are the compatibility contract.

## Observability

- `wrangler tail` streams worker logs (`log.Printf` output: webhook
  processing errors, cron sweep reports).
- `GET /healthz` is the liveness probe; `getWebhookInfo`
  (printed by `make webhook-setup`) shows Telegram-side delivery errors.
