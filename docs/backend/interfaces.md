# Interfaces Layer

`internal/interfaces/` adapts the outside world to the application layer:
Telegram updates (`bot`), the webhook HTTP endpoint (`http`), and the Cron
Trigger (`cron`). Handlers are thin: parse → call an application service →
render the reply from `texts.go`.

## `bot` — update handlers

`Handler.HandleUpdate` routes each `models.Update`:

- **New chat members** → `CaptchaService.OnMemberJoined`; depending on
  settings, send the captcha (buttons, or image + buttons) or the welcome
  message. Members in one join event are processed independently.
- **Callback queries** → dispatched by data prefix (see below).
- **Commands** → `parseCommand` lower-cases the name, strips any `@botname`
  suffix, splits args; `commandTarget` ignores commands addressed at other
  bots in the same group. Routed in `handleCommand` to one `cmd*` method per
  command (`/xqt`, `/invite`, `/filter`, `/captcha`, `/kick` `/ban` `/mute`
  `/unmute`, `/autoreact`, `/summary`, `/clean`, `/welcome`, `/roll`,
  `/pick`, `/start`, `/help`).
- **Ordinary group text** → `GroupMessagePipeline.HandleMessage`.

Rules:

- No Telegram Bot API calls here directly — everything goes through
  `ports.TelegramGateway`, so handlers stay testable and the layer stays
  transport-agnostic.
- Argument parsing lives in `parse.go` as pure functions (`splitPatternArg`
  treats `/…/`-wrapped input as regex, `parseAutoReactArgs` takes the last
  token as the emoji, `parseMinutes` applies defaults). These are unit-tested
  in `parse_test.go`.
- Admin-only actions check `IsAdmin` inside the application services; the
  handler just maps the resulting sentinel error.

## Callback-data protocol

Inline keyboards carry tiny ASCII payloads (Telegram limits callback data to
64 bytes), encoded/decoded in `parse.go`:

| Prefix | Format | Meaning |
|--------|--------|---------|
| `c:` | `c:<userID>:<optionIndex>` | captcha answer tap; only `<userID>` may answer it |
| `m:` | `m:<action>` | admin-panel toggle; actions are `cap`, `filter`, `llm`, `sum`, `refresh` |

Panel actions are admin-checked, applied via the matching service, and the
panel message is re-rendered in place (`EditText`).

## `texts.go` — all user-facing copy

Every user-facing string is Simplified Chinese and lives in exactly one file:
`internal/interfaces/bot/texts.go`. Constants ending in `T` are
`fmt.Sprintf` templates with a comment stating each placeholder. Never
hardcode Chinese copy in handlers or services; application services return
sentinel errors instead.

`errorText(err)` in `handler.go` is the single error-mapping point:
`application.ErrNotAdmin` → `textErrNotAdmin`, `ErrLLMNotConfigured` →
`textErrLLMNotConfigured`, …, unknown errors are logged and render the
generic `textErrUnknown`.

## `http` — webhook mux

`NewMux(cfg, handler)` registers:

- `POST /webhook` — requires header `X-Telegram-Bot-Api-Secret-Token` to
  match `cfg.WebhookSecret` (constant-time compare; the secret is registered
  with Telegram by `scripts/setup-webhook.sh`). Decodes `models.Update`,
  calls the handler, and **always answers 200** — a non-200 would make
  Telegram retry the update and storm the worker, so processing errors are
  only logged.
- `GET /healthz` — liveness (`ok`).
- `GET /` — one-line description.

There is deliberately no other HTTP surface (no REST API, no auth scheme
beyond the webhook secret).

## `cron` — scheduled adapter

`RunOnce(ctx, runner)` executes one `TaskRunner` sweep and logs the
`RunReport` (expired captchas, summaries sent, zombies kicked, per-task
errors). It has no js/wasm imports; the platform glue in `main_js.go` wraps
it with `cron.ScheduleTaskNonBlock`, keeping this package compilable and
testable on the host.
