// Package channelpost contains the domain rules for the channel-forwarding
// feature: comment previews, the bounded comment log, and the t.me link
// builders for posts and comments. Everything here is a pure function or
// value type so it can be unit-tested without any Telegram dependency.
package channelpost

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

// CommentPreviewCapacity caps how many comment previews are kept per post.
const CommentPreviewCapacity = 5

// commentTextRunes caps one comment preview's text length.
const commentTextRunes = 20

// Comment is one discussion-group reply to a forwarded channel post.
type Comment struct {
	MessageID int       `json:"message_id"` // the comment's message ID inside the discussion group
	Author    string    `json:"author"`
	Text      string    `json:"text"` // already trimmed to the preview length
	At        time.Time `json:"at"`
}

// ForwardedPost maps a channel post to its copied message in the bound group.
type ForwardedPost struct {
	ChannelID      int64 `json:"channel_id"`
	PostID         int   `json:"post_id"`
	GroupID        int64 `json:"group_id"`
	GroupMessageID int   `json:"group_message_id"`
}

// CommentLog is a bounded, oldest-first list of comment previews.
type CommentLog struct {
	Comments []Comment `json:"comments"`
}

// Add appends one comment, trimming its text to the preview length and
// evicting the oldest entries once the log exceeds its capacity.
func (l *CommentLog) Add(c Comment) {
	c.Text = PreviewText(c.Text)
	l.Comments = append(l.Comments, c)
	if len(l.Comments) > CommentPreviewCapacity {
		l.Comments = l.Comments[len(l.Comments)-CommentPreviewCapacity:]
	}
}

// PreviewText trims s to at most commentTextRunes runes, appending an
// ellipsis when the text was cut.
func PreviewText(s string) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= commentTextRunes {
		return string(r)
	}
	return string(r[:commentTextRunes]) + "…"
}

// PostLink returns the t.me link to one channel post.
func PostLink(cfg chat.ChannelConfig, postID int) string {
	return MessageLink(cfg.ChannelUsername, cfg.ChannelID, postID)
}

// CommentPageLink returns the link that opens a post's comment section.
func CommentPageLink(cfg chat.ChannelConfig, postID int) string {
	return PostLink(cfg, postID) + "?comment=1"
}

// CommentLink returns the t.me link to one comment inside the discussion
// group.
func CommentLink(cfg chat.ChannelConfig, commentMessageID int) string {
	return MessageLink(cfg.LinkedGroupUsername, cfg.LinkedGroupID, commentMessageID)
}

// MessageLink builds a t.me message link: public chats use their username,
// private ones the numeric ID without the -100 prefix.
func MessageLink(username string, chatID int64, messageID int) string {
	if username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", username, messageID)
	}
	return fmt.Sprintf("https://t.me/c/%s/%d", shortID(chatID), messageID)
}

// shortID strips the -100 prefix Telegram adds to supergroup/channel IDs.
func shortID(chatID int64) string {
	return strings.TrimPrefix(strconv.FormatInt(chatID, 10), "-100")
}
