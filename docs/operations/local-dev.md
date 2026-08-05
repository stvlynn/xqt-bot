# Local Development

Two ways to run the bot locally. Both use the same `setup()` wiring as
production; only the platform shims differ.

## Option A: host mode (`go run .`)

The host entrypoint (`main_host.go`, build tag `!js || !wasm`) is a plain
Go HTTP server with an **in-memory KV store** — state resets on restart and
no Cloudflare account is needed.

```sh
TELEGRAM_BOT_TOKEN=123:abc \
TELEGRAM_WEBHOOK_SECRET=local-secret \
BOT_USERNAME=yourbot \
LLM_API_KEY=sk-... \
go run .
# xqt-bot listening on :8787 (POST /webhook)
```

`LLM_*` are optional; without them AI features reply "not configured".

To receive real Telegram updates, expose `:8787` through a tunnel and point
the webhook at it (same contract as `scripts/setup-webhook.sh`):

```sh
cloudflared tunnel --url http://localhost:8787
curl -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/setWebhook" \
  -d url="https://<tunnel-host>/webhook" -d secret_token="local-secret"
```

Or skip Telegram entirely and post a synthetic update yourself — the header
must match the configured secret:

```sh
curl -X POST http://localhost:8787/webhook \
  -H 'Content-Type: application/json' \
  -H 'X-Telegram-Bot-Api-Secret-Token: local-secret' \
  -d '{"update_id": 1, "message": {"message_id": 1, "date": 0,
       "chat": {"id": 123, "type": "private"},
       "from": {"id": 42, "is_bot": false, "first_name": "T"},
       "text": "/start"}}'
```

Host mode serves no cron: `main_host.go` ignores the `TaskRunner`. Use
option B to exercise scheduled tasks.

## Option B: `wrangler dev`

Runs the real wasm worker locally, including the Cron Trigger and a local
preview of the KV namespace.

1. `cp .dev.vars.example .dev.vars` and fill in the values
   (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`, optional `LLM_API_KEY`).
   `.dev.vars` is git-ignored — never commit it.
2. `make dev` (i.e. `wrangler dev`). The worker listens on
   `http://localhost:8787`; KV operations hit the local preview namespace
   (`preview_id` in `wrangler.toml`), so dev data never touches production.
3. Trigger a scheduled sweep by hand:

   ```sh
   curl "http://localhost:8787/cdn-cgi/handler/scheduled"
   ```

   Watch the worker logs for the `cron sweep: ...` report line.

Notes:

- The first `wrangler dev` run compiles the wasm bundle; rebuild with
  `make build-wasm` if you edit Go files while it is not watching.
- Telegram still needs a public URL to reach the webhook — use a tunnel as
  in option A, or use `wrangler dev --remote` cautiously (it talks to the
  real preview namespace).

## Common issues

- **`config: TELEGRAM_BOT_TOKEN is required`** — env var missing (host mode)
  or `.dev.vars` absent (wrangler mode).
- **401 from `/webhook`** — the `X-Telegram-Bot-Api-Secret-Token` header
  does not match `TELEGRAM_WEBHOOK_SECRET`.
- **`Illegal invocation` on outbound HTTP** — you added an ad-hoc
  `http.Client` in js/wasm code; use the shared client from
  `httpclient_js.go` (see `docs/decisions/0002-workers-fetch-transport.md`).
