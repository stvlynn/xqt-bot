package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

func TestInviteHandleStartInvalidPayload(t *testing.T) {
	svc := NewInviteService(newFakeSettingsRepo(), newFakeTelegram(), "xqt_bot")
	for _, payload := range []string{"", "abc", "j", "jnotanumber"} {
		if _, err := svc.HandleStart(context.Background(), 1, payload); !errors.Is(err, ErrInvalidPayload) {
			t.Fatalf("payload %q: want ErrInvalidPayload, got %v", payload, err)
		}
	}
}

func TestInviteHandleStartCreatesOneTimeLink(t *testing.T) {
	repo := newFakeSettingsRepo()
	tg := newFakeTelegram()
	svc := NewInviteService(repo, tg, "xqt_bot")
	svc.now = fixedClock

	st := chat.Default(-100123, "Test Group")
	st.Invite.ExpireMinutes = 15
	repo.seed(st)

	res, err := svc.HandleStart(context.Background(), 42, "j-100123")
	if err != nil {
		t.Fatalf("HandleStart: %v", err)
	}
	if res.ChatID != -100123 || res.ChatTitle != "Test Group" || res.URL != tg.inviteURL || res.ExpireMinutes != 15 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(tg.inviteCalls) != 1 {
		t.Fatalf("want 1 invite call, got %d", len(tg.inviteCalls))
	}
	call := tg.inviteCalls[0]
	if call.memberLimit != 1 {
		t.Fatalf("want member limit 1, got %d", call.memberLimit)
	}
	if want := fixedNow.Add(15 * time.Minute); !call.expireAt.Equal(want) {
		t.Fatalf("want expiry %v, got %v", want, call.expireAt)
	}
}

func TestInviteHandleStartUnknownChatUsesDefaults(t *testing.T) {
	tg := newFakeTelegram()
	svc := NewInviteService(newFakeSettingsRepo(), tg, "xqt_bot")
	svc.now = fixedClock

	res, err := svc.HandleStart(context.Background(), 1, "j-100999")
	if err != nil {
		t.Fatalf("HandleStart: %v", err)
	}
	if res.ExpireMinutes != 10 { // domain default
		t.Fatalf("want default 10 minutes, got %d", res.ExpireMinutes)
	}
}

func TestInviteCreateShareLinkRequiresAdmin(t *testing.T) {
	tg := newFakeTelegram()
	svc := NewInviteService(newFakeSettingsRepo(), tg, "xqt_bot")

	if _, err := svc.CreateShareLink(context.Background(), -100123, 7); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}

	tg.setAdmin(-100123, 7, true)
	link, err := svc.CreateShareLink(context.Background(), -100123, 7)
	if err != nil {
		t.Fatalf("CreateShareLink: %v", err)
	}
	if want := "https://t.me/xqt_bot?start=j-100123"; link != want {
		t.Fatalf("want %q, got %q", want, link)
	}
}

func TestInviteHandleStartBackfillsTitleFromTelegram(t *testing.T) {
	repo := newFakeSettingsRepo()
	tg := newFakeTelegram()
	tg.chatTitle = "后端技术交流"
	svc := NewInviteService(repo, tg, "xqt_bot")
	svc.now = fixedClock

	// Stored settings with an empty title, as materialized on first sight.
	repo.seed(chat.Default(-100777, ""))

	res, err := svc.HandleStart(context.Background(), 42, "j-100777")
	if err != nil {
		t.Fatalf("HandleStart: %v", err)
	}
	if res.ChatTitle != "后端技术交流" {
		t.Fatalf("want backfilled title, got %q", res.ChatTitle)
	}
	stored, err := repo.Get(context.Background(), -100777)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != "后端技术交流" {
		t.Fatalf("title not persisted: %q", stored.Title)
	}
}
