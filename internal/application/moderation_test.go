package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
)

func setupModeration() (*ModerationService, *fakeSettingsRepo, *fakeTelegram) {
	repo := newFakeSettingsRepo()
	tg := newFakeTelegram()
	svc := NewModerationService(repo, tg)
	svc.now = fixedClock
	return svc, repo, tg
}

func TestCheckMessageHitDeletesAndMutes(t *testing.T) {
	svc, repo, tg := setupModeration()
	st := chat.Default(-1, "")
	st.Filter.Enabled = true
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	st.Filter.MuteMinutes = 10
	st.Filter.DeleteMessage = true
	repo.seed(st)

	res, err := svc.CheckMessage(context.Background(), -1, 42, 100, "this is SPAM text")
	if err != nil {
		t.Fatalf("CheckMessage: %v", err)
	}
	if !res.Hit || !res.Deleted {
		t.Fatalf("want hit+deleted, got %+v", res)
	}
	if res.Rule.Pattern != "spam" {
		t.Fatalf("unexpected rule: %+v", res.Rule)
	}
	if want := fixedNow.Add(10 * time.Minute); !res.MutedUntil.Equal(want) {
		t.Fatalf("want muted until %v, got %v", want, res.MutedUntil)
	}
	if len(tg.deleted) != 1 || tg.deleted[0].messageID != 100 {
		t.Fatalf("want message 100 deleted, got %+v", tg.deleted)
	}
	if len(tg.restrictions) != 1 || tg.restrictions[0].canSend {
		t.Fatalf("want one mute, got %+v", tg.restrictions)
	}
}

func TestCheckMessageNoHit(t *testing.T) {
	svc, repo, tg := setupModeration()
	st := chat.Default(-1, "")
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	repo.seed(st)

	res, err := svc.CheckMessage(context.Background(), -1, 42, 100, "clean text")
	if err != nil {
		t.Fatalf("CheckMessage: %v", err)
	}
	if res.Hit {
		t.Fatalf("want no hit, got %+v", res)
	}
	if len(tg.deleted) != 0 || len(tg.restrictions) != 0 {
		t.Fatalf("expected no telegram actions")
	}
}

func TestCheckMessageFilterDisabled(t *testing.T) {
	svc, repo, tg := setupModeration()
	st := chat.Default(-1, "")
	st.Filter.Enabled = false
	repo.seed(st) // builtin rules would hit, but filter is off

	res, err := svc.CheckMessage(context.Background(), -1, 42, 100, "usdt 代充")
	if err != nil {
		t.Fatalf("CheckMessage: %v", err)
	}
	if res.Hit {
		t.Fatalf("want no hit when filter disabled")
	}
	if len(tg.deleted) != 0 {
		t.Fatalf("expected no telegram actions")
	}
}

func TestCheckMessageDeleteOnlyWhenMuteZero(t *testing.T) {
	svc, repo, tg := setupModeration()
	st := chat.Default(-1, "")
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	st.Filter.MuteMinutes = 0
	st.Filter.DeleteMessage = true
	repo.seed(st)

	res, err := svc.CheckMessage(context.Background(), -1, 42, 100, "spam")
	if err != nil {
		t.Fatalf("CheckMessage: %v", err)
	}
	if !res.Hit || !res.Deleted || !res.MutedUntil.IsZero() {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(tg.restrictions) != 0 {
		t.Fatalf("want no mute, got %+v", tg.restrictions)
	}
}

func TestAddRuleValidationAndDuplicate(t *testing.T) {
	svc, repo, tg := setupModeration()
	tg.setAdmin(-1, 7, true)

	if err := svc.AddWordRule(context.Background(), -1, 8, "word"); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	if err := svc.AddWordRule(context.Background(), -1, 7, "   "); err == nil {
		t.Fatalf("want validation error for empty word")
	}
	if err := svc.AddRegexRule(context.Background(), -1, 7, "("); err == nil {
		t.Fatalf("want validation error for bad regex")
	}
	if err := svc.AddWordRule(context.Background(), -1, 7, "casino"); err != nil {
		t.Fatalf("AddWordRule: %v", err)
	}
	if err := svc.AddWordRule(context.Background(), -1, 7, "casino"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}
	// Same pattern with a different kind is not a duplicate.
	if err := svc.AddRegexRule(context.Background(), -1, 7, "casino"); err != nil {
		t.Fatalf("AddRegexRule: %v", err)
	}

	st, _ := repo.Get(context.Background(), -1)
	var kinds []moderation.RuleKind
	for _, r := range st.Filter.Rules {
		if r.Pattern == "casino" {
			kinds = append(kinds, r.Kind)
		}
	}
	if len(kinds) != 2 {
		t.Fatalf("want 2 casino rules, got %v", kinds)
	}
}

func TestRemoveRule(t *testing.T) {
	svc, repo, tg := setupModeration()
	tg.setAdmin(-1, 7, true)
	st := chat.Default(-1, "")
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	repo.seed(st)

	if err := svc.RemoveRule(context.Background(), -1, 7, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := svc.RemoveRule(context.Background(), -1, 7, "spam"); err != nil {
		t.Fatalf("RemoveRule: %v", err)
	}
	got, _ := repo.Get(context.Background(), -1)
	if len(got.Filter.Rules) != 0 {
		t.Fatalf("want no rules left, got %+v", got.Filter.Rules)
	}
}

func TestSetFilterEnabledAndMuteMinutes(t *testing.T) {
	svc, repo, tg := setupModeration()
	tg.setAdmin(-1, 7, true)

	if err := svc.SetMuteMinutes(context.Background(), -1, 7, -5); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
	if err := svc.SetFilterEnabled(context.Background(), -1, 7, false); err != nil {
		t.Fatalf("SetFilterEnabled: %v", err)
	}
	if err := svc.SetMuteMinutes(context.Background(), -1, 7, 30); err != nil {
		t.Fatalf("SetMuteMinutes: %v", err)
	}
	st, _ := repo.Get(context.Background(), -1)
	if st.Filter.Enabled || st.Filter.MuteMinutes != 30 {
		t.Fatalf("unexpected settings: %+v", st.Filter)
	}
}

func TestKickBanMuteUnmuteGuards(t *testing.T) {
	svc, _, tg := setupModeration()
	tg.setAdmin(-1, 7, true) // requester admin
	tg.setAdmin(-1, 9, true) // target admin
	ctx := context.Background()

	if err := svc.Kick(ctx, -1, 8, 42); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	if err := svc.Kick(ctx, -1, 7, 9); !errors.Is(err, ErrTargetIsAdmin) {
		t.Fatalf("want ErrTargetIsAdmin, got %v", err)
	}

	if err := svc.Kick(ctx, -1, 7, 42); err != nil {
		t.Fatalf("Kick: %v", err)
	}
	if len(tg.bans) != 1 || len(tg.unbans) != 1 {
		t.Fatalf("kick = ban+unban, got bans=%v unbans=%v", tg.bans, tg.unbans)
	}

	if err := svc.Ban(ctx, -1, 7, 43); err != nil {
		t.Fatalf("Ban: %v", err)
	}
	if len(tg.bans) != 2 || len(tg.unbans) != 1 {
		t.Fatalf("ban must not unban, got bans=%v unbans=%v", tg.bans, tg.unbans)
	}

	if err := svc.Mute(ctx, -1, 7, 44, 0); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
	if err := svc.Mute(ctx, -1, 7, 44, 60); err != nil {
		t.Fatalf("Mute: %v", err)
	}
	if err := svc.Unmute(ctx, -1, 7, 44); err != nil {
		t.Fatalf("Unmute: %v", err)
	}
	if len(tg.restrictions) != 2 {
		t.Fatalf("want mute+unmute restrictions, got %+v", tg.restrictions)
	}
	if tg.restrictions[0].canSend || !tg.restrictions[1].canSend {
		t.Fatalf("unexpected restrictions: %+v", tg.restrictions)
	}
	if want := fixedNow.Add(time.Hour); !tg.restrictions[0].until.Equal(want) {
		t.Fatalf("want mute until %v, got %v", want, tg.restrictions[0].until)
	}
}
