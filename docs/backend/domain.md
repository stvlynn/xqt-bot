# Domain Layer

`internal/domain/` holds the business rules. It imports nothing from
`application`, `infrastructure`, or `interfaces` — no Telegram SDK, no KV
client, no `syscall/js`.

## Aggregates and entities

### `chat.Settings` — the only aggregate root

One aggregate per Telegram chat (`internal/domain/chat/settings.go`), guarded
by group administrators. It bundles all per-chat config as nested value
objects:

```go
type Settings struct {
    ChatID    int64
    Title     string
    Captcha   CaptchaConfig   // enabled, mode (button|image), timeout
    Filter    FilterConfig    // enabled, []moderation.FilterRule, mute minutes
    AutoReact AutoReactConfig // []reaction.Rule, LLM toggle
    Summary   SummaryConfig   // auto toggle, interval hours, max messages
    Welcome   WelcomeConfig   // text with {name}/{chat} placeholders
    Invite    InviteConfig    // link validity window
    Zombie    ZombieConfig    // inactivity threshold in days
}
```

`chat.Default(chatID, title)` defines out-of-the-box behavior: safe things on
(filter enabled with the built-in rules, delete + 10-minute mute), disruptive
things off (captcha, auto-summary, welcome, LLM reactions). The KV repository
materializes `Default` on first contact with a chat.

### `moderation`

- `FilterRule{Kind, Pattern}` — `word` (case-insensitive substring) or
  `regex` (RE2). Constructors `NewWordRule` / `NewRegexRule` validate eagerly
  (reject empty words and uncompilable patterns at write time).
  `MatchAny(rules, text)` returns the first hit.
- `BuiltinRules()` — the built-in ad/scam library (Chinese spam words,
  `t.me/joinchat` links, phone-number harvesting regexes) seeded into every
  new chat's settings.
- `Challenge` / `Session` — the join captcha. `NewChallenge(rng)` builds a
  language-neutral arithmetic question with shuffled near-miss options;
  `Session` adds chat/user/message IDs and `ExpiresAt`, with `Expired(now)`
  and `Correct(optionIndex)`.

### `reaction`

- `Rule{Kind, Pattern, Emoji}` — keyword (substring) or regex trigger mapped
  to an emoji. Constructors validate the emoji against `AllowedEmojis()`,
  the subset of Telegram's reaction whitelist we accept (also the only
  choices offered to the LLM).
- `Pick(rules, text)` selects the rule whose trigger appears **earliest** in
  the message ("first occurrence wins").

### Other packages

- `summary.Message` + `summary.Ring` — recorded text message and the bounded
  oldest-first buffer (`Append` evicts the oldest past capacity;
  `Since(t)` filters by time).
- `schedule.Task{Kind, ChatID, IntervalHours, NextRunAt}` — recurring task
  with `Due(now)` and `Rescheduled(now)`; kinds are `auto_summary` and
  `zombie_clean`.
- `invite` — deep-link payload codec: `EncodePayload(chatID)` → `j-100…`,
  `ParsePayload` back.

## Ports

`internal/domain/ports/` declares every interface the inner layers need;
infrastructure implements them:

- Repositories: `SettingsRepository`, `CaptchaRepository` (incl.
  `ListExpired` for the cron sweep), `MessageLogRepository`,
  `ActivityRepository` (last-seen per member), `TaskRepository`.
- Gateways: `TelegramGateway` (send/edit/delete messages, inline keyboards
  via `ports.Button`, invite links, restrict/ban, reactions, admin checks),
  `LLMGateway` (`Available`, `Summarize`, `PickReaction`), `ImageRenderer`
  (`RenderCaptcha` → PNG).
- `ports.ErrNotFound` is the single "missing entity" signal repositories
  return and services translate.

## Rules

- Validation of invariants happens here (e.g. regex rules are uncompilable
  only if written bypassing the constructors — persisted rules are validated
  on write).
- Domain code is pure Go: deterministic, clock and RNG injected by callers.
- JSON tags on domain types are the KV persistence format — changing them is
  a data migration; treat it accordingly.
