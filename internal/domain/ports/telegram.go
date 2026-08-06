package ports

import (
	"context"
	"time"
)

// Button is one inline-keyboard button. Exactly one of URL / Data is set:
// URL opens a link, Data is sent back as a callback query.
type Button struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
	Data string `json:"data,omitempty"`
}

// SendOpts are optional modifiers for an outgoing message.
type SendOpts struct {
	ReplyToMessageID   int
	Buttons            [][]Button
	DisableLinkPreview bool
}

// TelegramGateway abstracts the Telegram Bot API operations the application
// layer needs. It is implemented by the infrastructure telegram package.
type TelegramGateway interface {
	// SendText sends a text message and returns its message ID.
	SendText(ctx context.Context, chatID int64, text string, opts *SendOpts) (int, error)
	// SendPhoto uploads a PNG image with an optional caption.
	SendPhoto(ctx context.Context, chatID int64, png []byte, caption string, replyToMessageID int) error
	// EditText replaces the text (and buttons) of an existing message.
	EditText(ctx context.Context, chatID int64, messageID int, text string, buttons [][]Button) error
	// DeleteMessage removes a message.
	DeleteMessage(ctx context.Context, chatID int64, messageID int) error
	// AnswerCallback acknowledges a callback query; alert shows a modal.
	AnswerCallback(ctx context.Context, callbackID, text string, alert bool) error

	// CreateInviteLink creates a one-time invite link expiring at `expireAt`.
	CreateInviteLink(ctx context.Context, chatID int64, expireAt time.Time, memberLimit int) (string, error)

	// RestrictMember mutes (or un-mutes) a member until `until`.
	RestrictMember(ctx context.Context, chatID, userID int64, canSend bool, until time.Time) error
	// BanMember bans a member; revokeMessages also deletes their history.
	BanMember(ctx context.Context, chatID, userID int64, revokeMessages bool) error
	// UnbanMember lifts a ban (used for kick = ban + unban).
	UnbanMember(ctx context.Context, chatID, userID int64) error

	// SetReaction attaches an emoji reaction to a message.
	SetReaction(ctx context.Context, chatID int64, messageID int, emoji string) error

	// IsAdmin reports whether the user is an administrator of the chat.
	IsAdmin(ctx context.Context, chatID, userID int64) (bool, error)
	// BotIsAdmin reports whether the bot itself has admin rights in the chat.
	BotIsAdmin(ctx context.Context, chatID int64) (bool, error)
	// ChatTitle returns the chat's display title (empty for private chats).
	ChatTitle(ctx context.Context, chatID int64) (string, error)
}
