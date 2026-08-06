package application

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

var testChannelLabels = ChannelLabels{
	CommentsButton: func(count int) string {
		if count > 0 {
			return "comments(" + strconv.Itoa(count) + ")"
		}
		return "comments"
	},
	AnonymousAuthor: "anon",
}

type channelFixture struct {
	svc      *ChannelService
	settings *fakeSettingsRepo
	bindings *fakeChannelBindingRepo
	posts    *fakeForwardedPostRepo
	comments *fakeCommentLogRepo
	tg       *fakeTelegram
}

func newChannelFixture() *channelFixture {
	f := &channelFixture{
		settings: newFakeSettingsRepo(),
		bindings: newFakeChannelBindingRepo(),
		posts:    newFakeForwardedPostRepo(),
		comments: newFakeCommentLogRepo(),
		tg:       newFakeTelegram(),
	}
	f.svc = NewChannelService(f.settings, f.bindings, f.posts, f.comments, f.tg, testChannelLabels)
	return f
}

const (
	groupID     int64 = -500
	channelID   int64 = -100111
	linkedID    int64 = -100222
	requesterID int64 = 7
)

// seedChannel registers a public channel (with a linked discussion group) in
// the fake gateway.
func (f *channelFixture) seedChannel() {
	f.tg.chatInfos["@news"] = ports.ChatInfo{
		ID:           channelID,
		Title:        "News",
		Username:     "news",
		LinkedChatID: linkedID,
		IsChannel:    true,
	}
	f.tg.chatInfos[strconv.FormatInt(linkedID, 10)] = ports.ChatInfo{
		ID:       linkedID,
		Title:    "News Chat",
		Username: "newschat",
	}
}

func TestBindRequiresAdmin(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	_, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news")
	if !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("err = %v, want ErrNotAdmin", err)
	}
}

func TestBindRejectsNonChannel(t *testing.T) {
	f := newChannelFixture()
	f.tg.setAdmin(groupID, requesterID, true)
	f.tg.chatInfos["@somegroup"] = ports.ChatInfo{ID: -900, Title: "Group", IsChannel: false}
	_, err := f.svc.Bind(context.Background(), groupID, requesterID, "@somegroup")
	if !errors.Is(err, ErrNotAChannel) {
		t.Fatalf("err = %v, want ErrNotAChannel", err)
	}
}

func TestBindRejectsChannelLinkedHere(t *testing.T) {
	f := newChannelFixture()
	f.tg.setAdmin(groupID, requesterID, true)
	f.tg.chatInfos["@news"] = ports.ChatInfo{
		ID: channelID, Title: "News", IsChannel: true, LinkedChatID: groupID,
	}
	_, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news")
	if !errors.Is(err, ErrChannelLinkedHere) {
		t.Fatalf("err = %v, want ErrChannelLinkedHere", err)
	}
}

func TestBindRequiresBotChannelAdmin(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	f.tg.botNotAdmin[channelID] = true
	_, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news")
	if !errors.Is(err, ErrBotNotChannelAdmin) {
		t.Fatalf("err = %v, want ErrBotNotChannelAdmin", err)
	}
}

func TestBindUnknownChannel(t *testing.T) {
	f := newChannelFixture()
	f.tg.setAdmin(groupID, requesterID, true)
	_, err := f.svc.Bind(context.Background(), groupID, requesterID, "@ghost")
	if !errors.Is(err, ErrChannelNotFound) {
		t.Fatalf("err = %v, want ErrChannelNotFound", err)
	}
}

func TestBindWithPreviews(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)

	res, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news")
	if err != nil {
		t.Fatal(err)
	}
	if !res.PreviewsEnabled || res.ChannelTitle != "News" || res.ChannelUsername != "news" {
		t.Fatalf("result = %+v", res)
	}
	st, err := f.settings.Get(context.Background(), groupID)
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Channel
	if cfg.ChannelID != channelID || !cfg.PreviewsEnabled ||
		cfg.LinkedGroupID != linkedID || cfg.LinkedGroupUsername != "newschat" {
		t.Fatalf("settings.Channel = %+v", cfg)
	}
	if got, err := f.bindings.GetByChannel(context.Background(), channelID); err != nil || got != groupID {
		t.Fatalf("binding = %d, %v; want %d, nil", got, err, groupID)
	}
}

func TestBindWithoutPreviews(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	f.tg.botNotAdmin[linkedID] = true // bot not in the discussion group

	res, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news")
	if err != nil {
		t.Fatal(err)
	}
	if res.PreviewsEnabled {
		t.Fatal("PreviewsEnabled = true, want false")
	}
	st, err := f.settings.Get(context.Background(), groupID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Channel.PreviewsEnabled || st.Channel.LinkedGroupUsername != "" {
		t.Fatalf("settings.Channel = %+v", st.Channel)
	}
}

func TestUnbind(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Unbind(context.Background(), groupID, requesterID); err != nil {
		t.Fatal(err)
	}
	st, err := f.settings.Get(context.Background(), groupID)
	if err != nil {
		t.Fatal(err)
	}
	if st.Channel.Bound() {
		t.Fatalf("settings.Channel still bound: %+v", st.Channel)
	}
	if _, err := f.bindings.GetByChannel(context.Background(), channelID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("binding after unbind = %v, want ErrNotFound", err)
	}
}

func TestUnbindWithoutBinding(t *testing.T) {
	f := newChannelFixture()
	f.tg.setAdmin(groupID, requesterID, true)
	if err := f.svc.Unbind(context.Background(), groupID, requesterID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestHandleChannelPostUnboundSkips(t *testing.T) {
	f := newChannelFixture()
	if err := f.svc.HandleChannelPost(context.Background(), channelID, 42); err != nil {
		t.Fatal(err)
	}
	if len(f.tg.copies) != 0 {
		t.Fatalf("copies = %v, want none", f.tg.copies)
	}
}

func TestHandleChannelPostCopiesAndMaps(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}

	if err := f.svc.HandleChannelPost(context.Background(), channelID, 42); err != nil {
		t.Fatal(err)
	}
	if len(f.tg.copies) != 1 {
		t.Fatalf("copies = %v, want one", f.tg.copies)
	}
	cp := f.tg.copies[0]
	if cp.fromChatID != channelID || cp.toChatID != groupID || cp.messageID != 42 {
		t.Fatalf("copy = %+v", cp)
	}
	if len(cp.buttons) != 1 || len(cp.buttons[0]) != 1 {
		t.Fatalf("buttons = %+v, want a single row", cp.buttons)
	}
	btn := cp.buttons[0][0]
	if btn.Text != "comments" || btn.URL != "https://t.me/news/42?comment=1" {
		t.Fatalf("button = %+v", btn)
	}
	fp, err := f.posts.Get(context.Background(), channelID, 42)
	if err != nil {
		t.Fatal(err)
	}
	if fp.GroupID != groupID || fp.GroupMessageID != 1001 {
		t.Fatalf("mapping = %+v", fp)
	}
}

// commentMessage builds a discussion-group message that replies to the
// channel post's automatic forward.
func commentMessage(chatID int64, messageID int, text string) *models.Message {
	return &models.Message{
		ID:   messageID,
		Chat: models.Chat{ID: chatID, Type: models.ChatTypeSupergroup},
		Text: text,
		From: &models.User{FirstName: "Ada"},
		ReplyToMessage: &models.Message{
			IsAutomaticForward: true,
			ForwardOrigin: &models.MessageOrigin{
				Type: models.MessageOriginTypeChannel,
				MessageOriginChannel: &models.MessageOriginChannel{
					Chat:      models.Chat{ID: channelID},
					MessageID: 42,
				},
			},
		},
	}
}

func TestMaybeRecordCommentIgnoresNonComments(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}
	plain := &models.Message{ID: 1, Chat: models.Chat{ID: groupID}, Text: "hi"}
	if err := f.svc.MaybeRecordComment(context.Background(), plain); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.comments.List(context.Background(), channelID, 42); len(got) != 0 {
		t.Fatalf("comments = %v, want none", got)
	}
}

func TestMaybeRecordCommentSkipsWhenPreviewsDisabled(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	f.tg.botNotAdmin[linkedID] = true
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}
	m := commentMessage(linkedID, 9, "nice post")
	// The comment arrives in the discussion group, but previews are disabled
	// (the bot is not an admin there), so nothing is recorded.
	if err := f.svc.MaybeRecordComment(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.comments.List(context.Background(), channelID, 42); len(got) != 0 {
		t.Fatalf("comments = %v, want none", got)
	}
}

func TestMaybeRecordCommentRefreshesButtons(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HandleChannelPost(context.Background(), channelID, 42); err != nil {
		t.Fatal(err)
	}

	// The comment arrives in the channel's discussion group; the forwarded
	// message lives in the bound group.
	m := commentMessage(linkedID, 9, "this is a truly excellent post, thanks")
	if err := f.svc.MaybeRecordComment(context.Background(), m); err != nil {
		t.Fatal(err)
	}

	got, err := f.comments.List(context.Background(), channelID, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Author != "Ada" || got[0].Text != "this is a truly exce…" {
		t.Fatalf("comments = %+v", got)
	}
	if len(f.tg.buttonEdits) != 1 {
		t.Fatalf("button edits = %v, want one", f.tg.buttonEdits)
	}
	edit := f.tg.buttonEdits[0]
	if edit.chatID != groupID || edit.messageID != 1001 {
		t.Fatalf("edit = %+v", edit)
	}
	if len(edit.buttons) != 2 {
		t.Fatalf("edit buttons = %+v, want preview row + comments row", edit.buttons)
	}
	if edit.buttons[0][0].Text != "Ada: this is a truly exce…" ||
		edit.buttons[0][0].URL != "https://t.me/newschat/9" {
		t.Fatalf("preview button = %+v", edit.buttons[0][0])
	}
	if edit.buttons[1][0].Text != "comments(1)" ||
		edit.buttons[1][0].URL != "https://t.me/news/42?comment=1" {
		t.Fatalf("comments button = %+v", edit.buttons[1][0])
	}
}

func TestMaybeRecordCommentEditFailureIsSwallowed(t *testing.T) {
	f := newChannelFixture()
	f.seedChannel()
	f.tg.setAdmin(groupID, requesterID, true)
	if _, err := f.svc.Bind(context.Background(), groupID, requesterID, "@news"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.HandleChannelPost(context.Background(), channelID, 42); err != nil {
		t.Fatal(err)
	}
	f.tg.editErr = errors.New("message to edit not found")
	m := commentMessage(linkedID, 9, "great")
	if err := f.svc.MaybeRecordComment(context.Background(), m); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

func TestParseChannelRef(t *testing.T) {
	cases := map[string]any{
		"@news":                "@news",
		"news":                 "@news",
		"t.me/news":            "@news",
		"https://t.me/news":    "@news",
		"https://t.me/news/42": "@news",
		"-100111":              int64(-100111),
	}
	for in, want := range cases {
		got, err := parseChannelRef(in)
		if err != nil {
			t.Fatalf("parseChannelRef(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("parseChannelRef(%q) = %#v, want %#v", in, got, want)
		}
	}
	if _, err := parseChannelRef("  "); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("parseChannelRef(blank) = %v, want ErrInvalidArgument", err)
	}
}
