// Package ports declares the inward-facing interfaces (repositories and
// gateways) that the domain and application layers depend on. The
// infrastructure layer provides the concrete implementations.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// ErrNotFound is returned by repositories when an entity does not exist.
var ErrNotFound = errors.New("not found")

// SettingsRepository persists per-chat settings aggregates.
type SettingsRepository interface {
	Get(ctx context.Context, chatID int64) (*chat.Settings, error)
	Save(ctx context.Context, settings *chat.Settings) error
}

// CaptchaRepository persists pending join-captcha sessions.
type CaptchaRepository interface {
	Save(ctx context.Context, s *moderation.Session) error
	Get(ctx context.Context, chatID, userID int64) (*moderation.Session, error)
	Delete(ctx context.Context, chatID, userID int64) error
	// ListExpired returns every session that expired before `now`.
	// Implementations may cap the result size for one sweep.
	ListExpired(ctx context.Context, now time.Time) ([]moderation.Session, error)
}

// MessageLogRepository persists the bounded per-chat message ring used by
// summaries and zombie detection.
type MessageLogRepository interface {
	Append(ctx context.Context, chatID int64, m summary.Message) error
	Recent(ctx context.Context, chatID int64) ([]summary.Message, error)
}

// ActivityRepository tracks when each member was last seen (joined or spoke),
// which powers zombie-member cleanup.
type ActivityRepository interface {
	Touch(ctx context.Context, chatID, userID int64, at time.Time) error
	// LastSeen returns the last activity timestamp per tracked member.
	LastSeen(ctx context.Context, chatID int64) (map[int64]time.Time, error)
	Remove(ctx context.Context, chatID, userID int64) error
}

// TaskRepository persists recurring per-chat tasks.
type TaskRepository interface {
	// List returns all tasks across all chats (cron sweep granularity).
	List(ctx context.Context) ([]schedule.Task, error)
	Save(ctx context.Context, t schedule.Task) error
	Delete(ctx context.Context, kind schedule.Kind, chatID int64) error
}
