// Package telegram implements ports.TelegramGateway on top of
// github.com/go-telegram/bot. It is plain net/http code and compiles for
// both host (tests) and js/wasm (the worker).
package telegram

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// Gateway implements ports.TelegramGateway via the Bot API.
type Gateway struct {
	bot   *bot.Bot
	botID int64
}

// NewGateway wraps an initialized *bot.Bot. botID is the bot's own user ID
// (from getMe), needed for BotIsAdmin checks.
func NewGateway(b *bot.Bot, botID int64) *Gateway {
	return &Gateway{bot: b, botID: botID}
}

var _ ports.TelegramGateway = (*Gateway)(nil)

// buildMarkup converts port buttons into a Telegram inline keyboard.
// It returns nil when there are no buttons so callers can omit the markup.
func buildMarkup(buttons [][]ports.Button) *models.InlineKeyboardMarkup {
	if len(buttons) == 0 {
		return nil
	}
	rows := make([][]models.InlineKeyboardButton, 0, len(buttons))
	for _, row := range buttons {
		if len(row) == 0 {
			continue
		}
		r := make([]models.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			r = append(r, models.InlineKeyboardButton{
				Text:         b.Text,
				URL:          b.URL,
				CallbackData: b.Data,
			})
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// SendText implements ports.TelegramGateway.
func (g *Gateway) SendText(ctx context.Context, chatID int64, text string, opts *ports.SendOpts) (int, error) {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if opts != nil {
		if opts.ReplyToMessageID != 0 {
			params.ReplyParameters = &models.ReplyParameters{
				MessageID:                opts.ReplyToMessageID,
				AllowSendingWithoutReply: true,
			}
		}
		if markup := buildMarkup(opts.Buttons); markup != nil {
			params.ReplyMarkup = markup
		}
		if opts.DisableLinkPreview {
			disabled := true
			params.LinkPreviewOptions = &models.LinkPreviewOptions{IsDisabled: &disabled}
		}
	}
	msg, err := g.bot.SendMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("telegram: send message: %w", err)
	}
	return msg.ID, nil
}

// SendPhoto implements ports.TelegramGateway.
func (g *Gateway) SendPhoto(ctx context.Context, chatID int64, png []byte, caption string, replyToMessageID int) error {
	params := &bot.SendPhotoParams{
		ChatID:  chatID,
		Photo:   &models.InputFileUpload{Filename: "captcha.png", Data: bytes.NewReader(png)},
		Caption: caption,
	}
	if replyToMessageID != 0 {
		params.ReplyParameters = &models.ReplyParameters{
			MessageID:                replyToMessageID,
			AllowSendingWithoutReply: true,
		}
	}
	if _, err := g.bot.SendPhoto(ctx, params); err != nil {
		return fmt.Errorf("telegram: send photo: %w", err)
	}
	return nil
}

// EditText implements ports.TelegramGateway.
func (g *Gateway) EditText(ctx context.Context, chatID int64, messageID int, text string, buttons [][]ports.Button) error {
	params := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	}
	if markup := buildMarkup(buttons); markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := g.bot.EditMessageText(ctx, params); err != nil {
		return fmt.Errorf("telegram: edit message: %w", err)
	}
	return nil
}

// DeleteMessage implements ports.TelegramGateway.
func (g *Gateway) DeleteMessage(ctx context.Context, chatID int64, messageID int) error {
	_, err := g.bot.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: messageID,
	})
	if err != nil {
		return fmt.Errorf("telegram: delete message: %w", err)
	}
	return nil
}

// AnswerCallback implements ports.TelegramGateway.
func (g *Gateway) AnswerCallback(ctx context.Context, callbackID, text string, alert bool) error {
	_, err := g.bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       alert,
	})
	if err != nil {
		return fmt.Errorf("telegram: answer callback: %w", err)
	}
	return nil
}

// CreateInviteLink implements ports.TelegramGateway.
func (g *Gateway) CreateInviteLink(ctx context.Context, chatID int64, expireAt time.Time, memberLimit int) (string, error) {
	link, err := g.bot.CreateChatInviteLink(ctx, &bot.CreateChatInviteLinkParams{
		ChatID:      chatID,
		ExpireDate:  int(expireAt.Unix()),
		MemberLimit: memberLimit,
	})
	if err != nil {
		return "", fmt.Errorf("telegram: create invite link: %w", err)
	}
	return link.InviteLink, nil
}

// RestrictMember implements ports.TelegramGateway. When canSend is false
// every permission is denied (Telegram's default for a mute); when true the
// member is un-muted by granting the standard send permissions.
func (g *Gateway) RestrictMember(ctx context.Context, chatID, userID int64, canSend bool, until time.Time) error {
	permissions := &models.ChatPermissions{
		CanSendMessages:       canSend,
		CanSendAudios:         canSend,
		CanSendDocuments:      canSend,
		CanSendPhotos:         canSend,
		CanSendVideos:         canSend,
		CanSendVideoNotes:     canSend,
		CanSendVoiceNotes:     canSend,
		CanSendPolls:          canSend,
		CanSendOtherMessages:  canSend,
		CanAddWebPagePreviews: canSend,
	}
	params := &bot.RestrictChatMemberParams{
		ChatID:      chatID,
		UserID:      userID,
		Permissions: permissions,
		// Independent permissions avoid Telegram implicitly granting
		// related rights when only a subset is set.
		UseIndependentChatPermissions: true,
	}
	if !until.IsZero() {
		params.UntilDate = int(until.Unix())
	}
	_, err := g.bot.RestrictChatMember(ctx, params)
	if err != nil {
		return fmt.Errorf("telegram: restrict member: %w", err)
	}
	return nil
}

// BanMember implements ports.TelegramGateway.
func (g *Gateway) BanMember(ctx context.Context, chatID, userID int64, revokeMessages bool) error {
	_, err := g.bot.BanChatMember(ctx, &bot.BanChatMemberParams{
		ChatID:         chatID,
		UserID:         userID,
		RevokeMessages: revokeMessages,
	})
	if err != nil {
		return fmt.Errorf("telegram: ban member: %w", err)
	}
	return nil
}

// UnbanMember implements ports.TelegramGateway.
func (g *Gateway) UnbanMember(ctx context.Context, chatID, userID int64) error {
	_, err := g.bot.UnbanChatMember(ctx, &bot.UnbanChatMemberParams{
		ChatID:       chatID,
		UserID:       userID,
		OnlyIfBanned: true,
	})
	if err != nil {
		return fmt.Errorf("telegram: unban member: %w", err)
	}
	return nil
}

// SetReaction implements ports.TelegramGateway.
func (g *Gateway) SetReaction(ctx context.Context, chatID int64, messageID int, emoji string) error {
	_, err := g.bot.SetMessageReaction(ctx, &bot.SetMessageReactionParams{
		ChatID:    chatID,
		MessageID: messageID,
		Reaction: []models.ReactionType{
			{
				Type:              models.ReactionTypeTypeEmoji,
				ReactionTypeEmoji: &models.ReactionTypeEmoji{Emoji: emoji},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("telegram: set reaction: %w", err)
	}
	return nil
}

// IsAdmin implements ports.TelegramGateway.
func (g *Gateway) IsAdmin(ctx context.Context, chatID, userID int64) (bool, error) {
	member, err := g.bot.GetChatMember(ctx, &bot.GetChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return false, fmt.Errorf("telegram: get chat member: %w", err)
	}
	return isAdminStatus(member.Type), nil
}

// BotIsAdmin implements ports.TelegramGateway. When the bot's own ID was not
// known at construction time (botID == 0, e.g. on js/wasm where getMe cannot
// run during startup) it is resolved lazily on first use and cached.
func (g *Gateway) BotIsAdmin(ctx context.Context, chatID int64) (bool, error) {
	if g.botID == 0 {
		me, err := g.bot.GetMe(ctx)
		if err != nil {
			return false, fmt.Errorf("telegram: get me: %w", err)
		}
		g.botID = me.ID
	}
	return g.IsAdmin(ctx, chatID, g.botID)
}

// isAdminStatus reports whether a member status confers admin rights.
func isAdminStatus(t models.ChatMemberType) bool {
	return t == models.ChatMemberTypeOwner || t == models.ChatMemberTypeAdministrator
}

// ChatTitle implements ports.TelegramGateway.
func (g *Gateway) ChatTitle(ctx context.Context, chatID int64) (string, error) {
	info, err := g.bot.GetChat(ctx, &bot.GetChatParams{ChatID: chatID})
	if err != nil {
		return "", fmt.Errorf("telegram: get chat %d: %w", chatID, err)
	}
	return info.Title, nil
}

// CopyMessage implements ports.TelegramGateway.
func (g *Gateway) CopyMessage(ctx context.Context, fromChatID, toChatID int64, messageID int, buttons [][]ports.Button) (int, error) {
	params := &bot.CopyMessageParams{
		ChatID:     toChatID,
		FromChatID: fromChatID,
		MessageID:  messageID,
	}
	if markup := buildMarkup(buttons); markup != nil {
		params.ReplyMarkup = markup
	}
	copied, err := g.bot.CopyMessage(ctx, params)
	if err != nil {
		return 0, fmt.Errorf("telegram: copy message %d from %d to %d: %w", messageID, fromChatID, toChatID, err)
	}
	return copied.ID, nil
}

// EditButtons implements ports.TelegramGateway.
func (g *Gateway) EditButtons(ctx context.Context, chatID int64, messageID int, buttons [][]ports.Button) error {
	params := &bot.EditMessageReplyMarkupParams{
		ChatID:    chatID,
		MessageID: messageID,
	}
	if markup := buildMarkup(buttons); markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := g.bot.EditMessageReplyMarkup(ctx, params); err != nil {
		return fmt.Errorf("telegram: edit reply markup %d/%d: %w", chatID, messageID, err)
	}
	return nil
}

// ChatInfo implements ports.TelegramGateway.
func (g *Gateway) ChatInfo(ctx context.Context, chatRef any) (*ports.ChatInfo, error) {
	info, err := g.bot.GetChat(ctx, &bot.GetChatParams{ChatID: chatRef})
	if err != nil {
		return nil, fmt.Errorf("telegram: get chat %v: %w", chatRef, err)
	}
	return &ports.ChatInfo{
		ID:           info.ID,
		Title:        info.Title,
		Username:     info.Username,
		LinkedChatID: info.LinkedChatID,
		IsChannel:    info.Type == models.ChatTypeChannel,
	}, nil
}
