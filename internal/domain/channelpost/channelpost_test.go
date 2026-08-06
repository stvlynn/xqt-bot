package channelpost

import (
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

func TestCommentLogAddTrimsAndCaps(t *testing.T) {
	var log CommentLog
	for i := 0; i < CommentPreviewCapacity+2; i++ {
		log.Add(Comment{MessageID: i + 1, Author: "a", Text: "x", At: time.Now()})
	}
	if len(log.Comments) != CommentPreviewCapacity {
		t.Fatalf("len = %d, want %d", len(log.Comments), CommentPreviewCapacity)
	}
	// The two oldest comments were evicted.
	if log.Comments[0].MessageID != 3 {
		t.Fatalf("oldest kept MessageID = %d, want 3", log.Comments[0].MessageID)
	}
}

func TestPreviewText(t *testing.T) {
	short := "短消息"
	if got := PreviewText("  " + short + "  "); got != short {
		t.Fatalf("short text = %q, want %q", got, short)
	}
	long := []rune("这是一条非常非常长的评论内容，超过了二十个字符需要被截断处理")
	got := PreviewText(string(long))
	want := string(long[:20]) + "…"
	if got != want {
		t.Fatalf("long text = %q, want %q", got, want)
	}
}

func TestLinks(t *testing.T) {
	pub := chat.ChannelConfig{
		ChannelID:           -100111,
		ChannelUsername:     "news",
		LinkedGroupID:       -100222,
		LinkedGroupUsername: "newschat",
	}
	priv := chat.ChannelConfig{
		ChannelID:     -1001234567890,
		LinkedGroupID: -1009876543210,
	}

	if got := PostLink(pub, 42); got != "https://t.me/news/42" {
		t.Fatalf("public PostLink = %q", got)
	}
	if got := PostLink(priv, 42); got != "https://t.me/c/1234567890/42" {
		t.Fatalf("private PostLink = %q", got)
	}
	if got := CommentPageLink(pub, 42); got != "https://t.me/news/42?comment=1" {
		t.Fatalf("CommentPageLink = %q", got)
	}
	if got := CommentLink(pub, 7); got != "https://t.me/newschat/7" {
		t.Fatalf("public CommentLink = %q", got)
	}
	if got := CommentLink(priv, 7); got != "https://t.me/c/9876543210/7" {
		t.Fatalf("private CommentLink = %q", got)
	}
}
