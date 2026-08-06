// Package ports declares the inward-facing interfaces (repositories and
// gateways) that the domain and application layers depend on. The
// infrastructure layer provides the concrete implementations.
package ports

import (
	"context"
	"errors"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
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

// ChannelBindingRepository maps a channel to the group its posts are
// forwarded into. A channel forwards to at most one group.
type ChannelBindingRepository interface {
	Set(ctx context.Context, channelID, groupID int64) error
	// GetByChannel returns the bound group, or ports.ErrNotFound.
	GetByChannel(ctx context.Context, channelID int64) (int64, error)
	Delete(ctx context.Context, channelID int64) error
}

// ForwardedPostRepository records which group message mirrors a channel
// post, so later comments can update that message's buttons.
type ForwardedPostRepository interface {
	Save(ctx context.Context, p channelpost.ForwardedPost) error
	// Get returns the mapping, or ports.ErrNotFound.
	Get(ctx context.Context, channelID int64, postID int) (*channelpost.ForwardedPost, error)
}

// CommentLogRepository stores per-post comment previews.
type CommentLogRepository interface {
	Append(ctx context.Context, channelID int64, postID int, c channelpost.Comment) error
	// List returns the recorded previews, oldest first; an unknown post
	// yields an empty slice (no error).
	List(ctx context.Context, channelID int64, postID int) ([]channelpost.Comment, error)
}
