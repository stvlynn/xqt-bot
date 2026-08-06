package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

func setupSettings() (*SettingsService, *fakeSettingsRepo, *fakeTelegram) {
	repo := newFakeSettingsRepo()
	tg := newFakeTelegram()
	return NewSettingsService(repo, tg), repo, tg
}

func TestSettingsGetReturnsDefaultWithTitle(t *testing.T) {
	svc, _, _ := setupSettings()
	st, err := svc.Get(context.Background(), -1, "My Group")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if st.ChatID != -1 || st.Title != "My Group" {
		t.Fatalf("unexpected default: %+v", st)
	}
	if st.Zombie.InactiveDays != 30 || st.Invite.ExpireMinutes != 10 {
		t.Fatalf("want domain defaults, got %+v", st)
	}
}

func TestSettingsGetPersisted(t *testing.T) {
	svc, repo, _ := setupSettings()
	stored := chat.Default(-1, "Stored")
	stored.Welcome.Text = "hi {name}"
	repo.seed(stored)

	st, err := svc.Get(context.Background(), -1, "Other")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if st.Title != "Stored" || st.Welcome.Text != "hi {name}" {
		t.Fatalf("want persisted settings, got %+v", st)
	}
}

func TestSettingsSettersRequireAdmin(t *testing.T) {
	svc, _, _ := setupSettings()
	ctx := context.Background()
	if err := svc.SetCaptchaEnabled(ctx, -1, 8, true); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	if err := svc.SetWelcome(ctx, -1, 8, "hello"); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
}

func TestSettingsSettersValidation(t *testing.T) {
	svc, _, tg := setupSettings()
	tg.setAdmin(-1, 7, true)
	ctx := context.Background()

	if err := svc.SetCaptchaMode(ctx, -1, 7, "carrier-pigeon"); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
	if err := svc.SetWelcome(ctx, -1, 7, "   "); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
	if err := svc.SetInviteExpireMinutes(ctx, -1, 7, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
	if err := svc.SetInviteExpireMinutes(ctx, -1, 7, 7*24*60+1); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
}

func TestSettingsSettersPersist(t *testing.T) {
	svc, repo, tg := setupSettings()
	tg.setAdmin(-1, 7, true)
	ctx := context.Background()

	steps := []error{
		svc.SetCaptchaEnabled(ctx, -1, 7, true),
		svc.SetCaptchaMode(ctx, -1, 7, chat.CaptchaModeImage),
		svc.SetWelcome(ctx, -1, 7, "welcome {name}"),
		svc.SetWelcomeEnabled(ctx, -1, 7, true),
		svc.SetInviteExpireMinutes(ctx, -1, 7, 60),
	}
	for i, err := range steps {
		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	st, _ := repo.Get(ctx, -1)
	if !st.Captcha.Enabled || st.Captcha.Mode != chat.CaptchaModeImage ||
		st.Welcome.Text != "welcome {name}" || !st.Welcome.Enabled ||
		st.Invite.ExpireMinutes != 60 {
		t.Fatalf("unexpected settings: %+v", st)
	}
}

func TestSettingsGetBackfillsAndPersistsTitle(t *testing.T) {
	repo := newFakeSettingsRepo()
	repo.seed(chat.Default(-100888, ""))
	svc := NewSettingsService(repo, newFakeTelegram())

	st, err := svc.Get(context.Background(), -100888, "产品讨论组")
	if err != nil {
		t.Fatal(err)
	}
	if st.Title != "产品讨论组" {
		t.Fatalf("want backfilled title, got %q", st.Title)
	}
	again, err := svc.Get(context.Background(), -100888, "")
	if err != nil {
		t.Fatal(err)
	}
	if again.Title != "产品讨论组" {
		t.Fatalf("title not persisted: %q", again.Title)
	}
}
