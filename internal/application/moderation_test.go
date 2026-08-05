package application

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
)

func setupModeration() (*ModerationService, *fakeSettingsRepo, *fakeTelegram) {
	svc, repo, tg, _, _ := setupWordList()
	return svc, repo, tg
}

func setupWordList() (*ModerationService, *fakeSettingsRepo, *fakeTelegram, *fakeTaskRepo, *fakeWordList) {
	repo := newFakeSettingsRepo()
	tasks := newFakeTaskRepo()
	tg := newFakeTelegram()
	wl := newFakeWordList()
	svc := NewModerationService(repo, tasks, tg, wl)
	svc.now = fixedClock
	return svc, repo, tg, tasks, wl
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

func TestImportWordList(t *testing.T) {
	svc, repo, tg, tasks, wl := setupWordList()
	ctx := context.Background()
	const url = "https://x/list.txt"

	if _, err := svc.ImportWordList(ctx, -1, 8, url); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	tg.setAdmin(-1, 7, true)

	// Manual rule collides with one fetched rule -> skipped.
	st := chat.Default(-1, "")
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	repo.seed(st)
	wl.rules[url] = wordRules(url, "spam", "scam", "phish")

	res, err := svc.ImportWordList(ctx, -1, 7, url)
	if err != nil {
		t.Fatalf("ImportWordList: %v", err)
	}
	if res.Added != 2 || res.Skipped != 1 || res.Total != 3 || res.URL != url {
		t.Fatalf("unexpected result: %+v", res)
	}
	got, _ := repo.Get(ctx, -1)
	if len(got.Filter.Sources) != 1 || got.Filter.Sources[0] != url {
		t.Fatalf("want source recorded, got %+v", got.Filter.Sources)
	}
	// Imported rules carry the source; the manual rule does not.
	for _, r := range got.Filter.Rules {
		if r.Pattern == "spam" && r.Source != "" {
			t.Fatalf("manual rule must keep empty source: %+v", r)
		}
		if r.Pattern != "spam" && r.Source != url {
			t.Fatalf("imported rule must carry source: %+v", r)
		}
	}
	// First source schedules the daily refresh task.
	list, _ := tasks.List(ctx)
	if len(list) != 1 || list[0].Kind != schedule.KindFilterRefresh ||
		!list[0].NextRunAt.Equal(fixedNow.Add(24*time.Hour)) {
		t.Fatalf("want filter_refresh task, got %+v", list)
	}

	// Re-importing the same URL replaces its old rules and creates no task.
	wl.rules[url] = wordRules(url, "scam", "malware")
	res, err = svc.ImportWordList(ctx, -1, 7, url)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if res.Added != 2 || res.Total != 3 { // manual spam + scam + malware
		t.Fatalf("unexpected re-import result: %+v", res)
	}
	got, _ = repo.Get(ctx, -1)
	var patterns []string
	for _, r := range got.Filter.Rules {
		patterns = append(patterns, r.Pattern)
	}
	if len(got.Filter.Sources) != 1 || len(patterns) != 3 {
		t.Fatalf("unexpected state after re-import: %+v", got.Filter)
	}
	list, _ = tasks.List(ctx)
	if len(list) != 1 {
		t.Fatalf("re-import must not create another task, got %+v", list)
	}
}

func TestImportWordListLimit(t *testing.T) {
	svc, repo, tg, _, wl := setupWordList()
	ctx := context.Background()
	const url = "https://x/big.txt"
	tg.setAdmin(-1, 7, true)

	st := chat.Default(-1, "")
	for i := 0; i < maxFilterRules; i++ {
		st.Filter.Rules = append(st.Filter.Rules, moderation.FilterRule{
			Kind: moderation.RuleWord, Pattern: fmt.Sprintf("w%d", i),
		})
	}
	repo.seed(st)
	wl.rules[url] = wordRules(url, "one-more")

	if _, err := svc.ImportWordList(ctx, -1, 7, url); err == nil {
		t.Fatalf("want limit error")
	}
	got, _ := repo.Get(ctx, -1)
	if len(got.Filter.Sources) != 0 || len(got.Filter.Rules) != maxFilterRules {
		t.Fatalf("failed import must not persist anything")
	}
}

func TestRefreshWordLists(t *testing.T) {
	svc, repo, tg, _, wl := setupWordList()
	ctx := context.Background()
	tg.setAdmin(-1, 7, true)

	if _, err := svc.RefreshWordLists(ctx, -1, 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound without sources, got %v", err)
	}

	const okURL, badURL = "https://x/ok.txt", "https://x/bad.txt"
	st := chat.Default(-1, "")
	st.Filter.Sources = []string{okURL, badURL}
	st.Filter.Rules = []moderation.FilterRule{
		{Kind: moderation.RuleWord, Pattern: "old", Source: okURL},
		{Kind: moderation.RuleWord, Pattern: "stale", Source: badURL},
		{Kind: moderation.RuleWord, Pattern: "manual"},
	}
	repo.seed(st)
	wl.rules[okURL] = wordRules(okURL, "new1", "new2")
	wl.errs[badURL] = errors.New("boom")

	res, err := svc.RefreshWordLists(ctx, -1, 7)
	if err != nil {
		t.Fatalf("RefreshWordLists: %v", err)
	}
	if res.Sources != 2 || len(res.Failed) != 1 || res.Failed[0] != badURL {
		t.Fatalf("unexpected result: %+v", res)
	}
	// "old" replaced by new1+new2; failed source keeps its stale rules.
	if res.Added != 1 { // 3 -> 4 rules
		t.Fatalf("want net +1, got %+v", res)
	}
	got, _ := repo.Get(ctx, -1)
	var patterns []string
	for _, r := range got.Filter.Rules {
		patterns = append(patterns, r.Pattern)
	}
	want := []string{"stale", "manual", "new1", "new2"}
	if fmt.Sprint(patterns) != fmt.Sprint(want) {
		t.Fatalf("want %v, got %v", want, patterns)
	}
}

func TestRefreshWordListsNotAdmin(t *testing.T) {
	svc, _, _, _, _ := setupWordList()
	if _, err := svc.RefreshWordLists(context.Background(), -1, 8); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
}

func TestRefreshAllWordListsNoSourcesIsNoop(t *testing.T) {
	svc, _, _, _, _ := setupWordList()
	res, err := svc.RefreshAllWordLists(context.Background(), -1)
	if err != nil {
		t.Fatalf("RefreshAllWordLists: %v", err)
	}
	if res.Sources != 0 || len(res.Failed) != 0 {
		t.Fatalf("want empty result, got %+v", res)
	}
}
