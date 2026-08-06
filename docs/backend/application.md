# Application Layer

`internal/application/` orchestrates use cases. It depends only on `domain`
and the `ports` interfaces; it never touches KV keys or HTTP. One deliberate
exception: `ChannelService.MaybeRecordComment` takes the Telegram
`models.Message` directly, because comment detection hinges on the
`MessageOrigin` union which has no domain counterpart.

## Services

One service per feature area, all constructed in `setup()` (`main.go`):

| Service | Use cases |
|---------|-----------|
| `CaptchaService` | member join → issue challenge, solve answer, bind/delete the captcha message, `SweepExpired` (kick no-shows) |
| `SettingsService` | read settings, toggle captcha/welcome, set captcha mode, set welcome text |
| `ModerationService` | filter rule add/remove, remote word-list import (`ImportWordList`, replaces rules of the same source, skips duplicates, 5000-rule cap, first import schedules `filter_refresh`) and refresh (`RefreshWordLists` / `RefreshAllWordLists`), toggle filter, `CheckMessage` (delete + mute on hit), `Kick`/`Ban`/`Mute`/`Unmute` |
| `InviteService` | `CreateShareLink` (`t.me/<bot>?start=j<chatID>`), `HandleStart` (resolve deep link → one-time invite URL) |
| `ReactionService` | auto-react rule management, LLM toggle, `OnMessage` (rule match → `SetReaction`, else LLM pick when enabled) |
| `SummaryService` | `RecordMessage` into the ring, `SummarizeNow`, `SetAutoSummary` (writes a `task:auto_summary:` entry) |
| `ZombieService` | `Touch` activity, `Preview`/`Clean` inactive members, `SetInactiveDays` |
| `FunService` | `/roll` d100 duel, `/pick` random choice |
| `ChannelService` | `Bind`/`Unbind` a channel to a group (admin-checked; refuses when the channel's discussion group is the binding chat), `HandleChannelPost` (copy the post into the bound group with a comments button, record the message mapping), `MaybeRecordComment` (detect discussion-group comments on the channel's auto-forward, store a ≤5-preview log, re-render the forwarded message's buttons) |

Plus two coordinators:

- **`GroupMessagePipeline`** (`pipeline.go`) — every inbound group text goes
  through a fixed order: activity touch → message log → moderation →
  auto-reaction (skipped when moderation hit). A failing step is aggregated
  via `errors.Join` and never blocks the remaining steps.
- **`TaskRunner`** (`tasks.go`) — one cron sweep: sweep expired captchas,
  list all tasks, run due ones (`auto_summary`, `zombie_clean`,
  `filter_refresh`), reschedule each executed task. Per-task failures land in
  `RunReport.Errors` and never abort the sweep.

## Rules

- **No business rules here.** Services delegate to domain entities
  (`FilterRule.Match`, `Session.Expired`, `Task.Due`) and enforce
  cross-cutting policy: admin checks (`IsAdmin` before any mutation), target
  validation (`ErrTargetIsAdmin`), LLM availability.
- **Ports only.** A service is constructed with `ports.*` interfaces plus
  plain values (e.g. `InviteService` takes `botUsername`). That is what makes
  services testable with the in-memory fakes in `fakes_test.go`.
- **Structured results, not prose.** Services return result structs
  (`SolveResult{Passed, Expired}`, `CleanResult{Kicked, Skipped}`) or
  sentinel errors. All user-facing wording is the interfaces layer's job.

## Sentinel errors

`errors.go` defines the stable error contract the interfaces layer maps to
Chinese copy (`errorText` in `internal/interfaces/bot/handler.go`):

```go
ErrInvalidPayload   // bad /start deep-link payload
ErrNotAdmin         // requester is not a chat admin
ErrTargetIsAdmin    // moderation action aimed at an admin
ErrDuplicate        // identical rule exists
ErrNotFound         // entity missing
ErrTooFewMessages   // not enough messages to summarize
ErrLLMNotConfigured // LLM feature without LLM_API_KEY
ErrInvalidArgument  // out-of-range or empty parameters
ErrChannelLinkedHere   // channel's discussion group is the binding chat
ErrChannelNotFound     // channel reference cannot be resolved
ErrNotAChannel         // /channel target is not a channel
ErrBotNotChannelAdmin  // bot is not an administrator of the channel
```

Add a sentinel only when the interfaces layer must render a distinct message;
otherwise return a wrapped descriptive error (it will be logged and shown as
the generic "something broke" reply).

## Testing

Services are unit-tested white-box with fakes implementing every port and an
injected fixed clock — see [`../quality/testing.md`](../quality/testing.md).
