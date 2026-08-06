package application

import (
	"context"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// ChannelLabels carries the user-facing button labels the service needs.
// They are injected by the interfaces layer so all copy stays in texts.go.
type ChannelLabels struct {
	// CommentsButton renders the "go to comments" button label; count is the
	// number of recorded comment previews (0 renders the plain label).
	CommentsButton func(count int) string
	// AnonymousAuthor labels comment authors Telegram hides.
	AnonymousAuthor string
}

// BindResult summarizes a successful /channel binding for the reply copy.
type BindResult struct {
	ChannelTitle    string
	ChannelUsername string
	PreviewsEnabled bool
}

// ChannelService binds channels to groups, forwards new channel posts into
// the bound group, and keeps the forwarded message's comment buttons up to
// date as comments arrive in the channel's discussion group.
type ChannelService struct {
	settings ports.SettingsRepository
	bindings ports.ChannelBindingRepository
	posts    ports.ForwardedPostRepository
	comments ports.CommentLogRepository
	tg       ports.TelegramGateway
	labels   ChannelLabels
	now      func() time.Time
}

// NewChannelService builds the service.
func NewChannelService(settings ports.SettingsRepository, bindings ports.ChannelBindingRepository, posts ports.ForwardedPostRepository, comments ports.CommentLogRepository, tg ports.TelegramGateway, labels ChannelLabels) *ChannelService {
	return &ChannelService{settings: settings, bindings: bindings, posts: posts, comments: comments, tg: tg, labels: labels, now: clockNow}
}

// Bind wires a channel to a group (/channel @name). The requester must be a
// group admin and the bot a channel admin. When the channel's discussion
// group is the requesting chat, Telegram already forwards natively and the
// bind is refused with ErrChannelLinkedHere.
func (s *ChannelService) Bind(ctx context.Context, groupID, requesterID int64, ref string) (*BindResult, error) {
	if err := requireAdmin(ctx, s.tg, groupID, requesterID); err != nil {
		return nil, err
	}
	chatRef, err := parseChannelRef(ref)
	if err != nil {
		return nil, err
	}
	info, err := s.tg.ChatInfo(ctx, chatRef)
	if err != nil {
		return nil, ErrChannelNotFound
	}
	if !info.IsChannel {
		return nil, ErrNotAChannel
	}
	if info.LinkedChatID == groupID {
		return nil, ErrChannelLinkedHere
	}
	if ok, err := s.tg.BotIsAdmin(ctx, info.ID); err != nil || !ok {
		// Telegram also errors when the bot is not in the channel at all;
		// either way the remedy is the same: add the bot as a channel admin.
		return nil, ErrBotNotChannelAdmin
	}

	// Comment previews require the bot to read the discussion group.
	previews := false
	linkedUsername := ""
	if info.LinkedChatID != 0 {
		if ok, err := s.tg.BotIsAdmin(ctx, info.LinkedChatID); err == nil && ok {
			previews = true
			if ginfo, err := s.tg.ChatInfo(ctx, info.LinkedChatID); err == nil {
				linkedUsername = ginfo.Username
			}
		}
	}

	st, err := loadSettings(ctx, s.settings, groupID)
	if err != nil {
		return nil, err
	}
	st.Channel = chat.ChannelConfig{
		ChannelID:           info.ID,
		ChannelTitle:        info.Title,
		ChannelUsername:     info.Username,
		LinkedGroupID:       info.LinkedChatID,
		LinkedGroupUsername: linkedUsername,
		PreviewsEnabled:     previews,
	}
	if err := s.settings.Save(ctx, st); err != nil {
		return nil, err
	}
	if err := s.bindings.Set(ctx, info.ID, groupID); err != nil {
		return nil, err
	}
	return &BindResult{
		ChannelTitle:    info.Title,
		ChannelUsername: info.Username,
		PreviewsEnabled: previews,
	}, nil
}

// Unbind removes the group's channel binding (/channel off).
func (s *ChannelService) Unbind(ctx context.Context, groupID, requesterID int64) error {
	if err := requireAdmin(ctx, s.tg, groupID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, groupID)
	if err != nil {
		return err
	}
	if !st.Channel.Bound() {
		return ErrNotFound
	}
	channelID := st.Channel.ChannelID
	st.Channel = chat.ChannelConfig{}
	if err := s.settings.Save(ctx, st); err != nil {
		return err
	}
	return s.bindings.Delete(ctx, channelID)
}

// HandleChannelPost forwards one new channel post into its bound group with
// a "go to comments" button and records the message mapping so later
// comments can update the buttons. Channels bound nowhere are skipped.
func (s *ChannelService) HandleChannelPost(ctx context.Context, channelID int64, postID int) error {
	groupID, err := s.bindings.GetByChannel(ctx, channelID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, groupID)
	if err != nil {
		return err
	}
	cfg := st.Channel
	if cfg.ChannelID != channelID {
		return nil // stale binding left over from an older configuration
	}
	buttons := [][]ports.Button{{
		{Text: s.labels.CommentsButton(0), URL: channelpost.CommentPageLink(cfg, postID)},
	}}
	groupMessageID, err := s.tg.CopyMessage(ctx, channelID, groupID, postID, buttons)
	if err != nil {
		return err
	}
	return s.posts.Save(ctx, channelpost.ForwardedPost{
		ChannelID:      channelID,
		PostID:         postID,
		GroupID:        groupID,
		GroupMessageID: groupMessageID,
	})
}

// MaybeRecordComment inspects one group message and, when it is a comment on
// a forwarded channel post (a reply to the channel's automatic forward in
// the discussion group), records a preview and refreshes the forwarded
// message's buttons. Anything else is ignored. Button-edit failures (the
// forwarded message was deleted, etc.) are logged, never reported.
//
// Note: this method takes the Telegram message model directly — a deliberate
// exception to the "application depends only on domain" rule, because
// comment detection hinges on the MessageOrigin union that has no domain
// counterpart.
func (s *ChannelService) MaybeRecordComment(ctx context.Context, m *models.Message) error {
	channelID, postID, ok := commentTarget(m)
	if !ok || m.Text == "" {
		return nil
	}
	// The comment arrives in the channel's discussion group; the forwarding
	// configuration lives on the bound group, found via the binding.
	groupID, err := s.bindings.GetByChannel(ctx, channelID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, groupID)
	if err != nil {
		return err
	}
	cfg := st.Channel
	if !cfg.Bound() || cfg.ChannelID != channelID || !cfg.PreviewsEnabled {
		return nil
	}
	if cfg.LinkedGroupID == 0 || m.Chat.ID != cfg.LinkedGroupID {
		return nil // not a comment in the channel's discussion group
	}
	if err := s.comments.Append(ctx, channelID, postID, channelpost.Comment{
		MessageID: m.ID,
		Author:    commentAuthor(m.From, s.labels.AnonymousAuthor),
		Text:      m.Text,
		At:        s.now(),
	}); err != nil {
		return err
	}
	fp, err := s.posts.Get(ctx, channelID, postID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil // forwarded before the mapping existed or already expired
	}
	if err != nil {
		return err
	}
	all, err := s.comments.List(ctx, channelID, postID)
	if err != nil {
		return err
	}
	buttons := make([][]ports.Button, 0, len(all)+1)
	for _, c := range all {
		buttons = append(buttons, []ports.Button{{
			Text: c.Author + ": " + c.Text,
			URL:  channelpost.CommentLink(cfg, c.MessageID),
		}})
	}
	buttons = append(buttons, []ports.Button{{
		Text: s.labels.CommentsButton(len(all)),
		URL:  channelpost.CommentPageLink(cfg, postID),
	}})
	if err := s.tg.EditButtons(ctx, fp.GroupID, fp.GroupMessageID, buttons); err != nil {
		log.Printf("channel: edit comment buttons chat %d message %d: %v", fp.GroupID, fp.GroupMessageID, err)
	}
	return nil
}

// commentTarget extracts the channel ID and post ID a message comments on,
// following the reply → automatic-forward → channel-origin chain.
func commentTarget(m *models.Message) (channelID int64, postID int, ok bool) {
	if m == nil || m.ReplyToMessage == nil || !m.ReplyToMessage.IsAutomaticForward {
		return 0, 0, false
	}
	origin := m.ReplyToMessage.ForwardOrigin
	if origin == nil || origin.Type != models.MessageOriginTypeChannel || origin.MessageOriginChannel == nil {
		return 0, 0, false
	}
	return origin.MessageOriginChannel.Chat.ID, origin.MessageOriginChannel.MessageID, true
}

// commentAuthor renders the comment's sender for the preview button.
func commentAuthor(u *models.User, anonymous string) string {
	if u == nil {
		return anonymous
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return anonymous
}

// parseChannelRef normalizes a /channel argument into a GetChat reference:
// "@name", "t.me/name" and "https://t.me/name" resolve by username, a bare
// number by chat ID.
func parseChannelRef(ref string) (any, error) {
	ref = strings.TrimSpace(ref)
	for _, prefix := range []string{"https://t.me/", "http://t.me/", "t.me/"} {
		ref = strings.TrimPrefix(ref, prefix)
	}
	ref, _, _ = strings.Cut(ref, "/") // drop any post path
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, ErrInvalidArgument
	}
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		return id, nil
	}
	ref = strings.TrimPrefix(ref, "@")
	if ref == "" {
		return nil, ErrInvalidArgument
	}
	return "@" + ref, nil
}
