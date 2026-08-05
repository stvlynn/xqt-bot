package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/reaction"
)

func setupReaction(llm *fakeLLM) (*ReactionService, *fakeSettingsRepo, *fakeTelegram) {
	repo := newFakeSettingsRepo()
	tg := newFakeTelegram()
	svc := NewReactionService(repo, tg, llm)
	svc.now = fixedClock
	return svc, repo, tg
}

func TestOnMessageRuleHit(t *testing.T) {
	svc, repo, tg := setupReaction(&fakeLLM{})
	st := chat.Default(-1, "")
	st.AutoReact.Rules = []reaction.Rule{{Kind: reaction.KindKeyword, Pattern: "nice", Emoji: "👍"}}
	repo.seed(st)

	if err := svc.OnMessage(context.Background(), -1, 100, "that is nice"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if len(tg.reactions) != 1 || tg.reactions[0].emoji != "👍" || tg.reactions[0].messageID != 100 {
		t.Fatalf("unexpected reactions: %+v", tg.reactions)
	}
}

func TestOnMessageNoRuleNoLLM(t *testing.T) {
	llm := &fakeLLM{available: true, reactionEmoji: "🔥", reactionOK: true}
	svc, _, tg := setupReaction(llm) // no rules, LLM fallback disabled

	if err := svc.OnMessage(context.Background(), -1, 100, "hello"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if len(tg.reactions) != 0 || llm.pickCalls != 0 {
		t.Fatalf("want no reaction and no LLM call")
	}
}

func TestOnMessageLLMFallback(t *testing.T) {
	llm := &fakeLLM{available: true, reactionEmoji: "🔥", reactionOK: true}
	svc, repo, tg := setupReaction(llm)
	st := chat.Default(-1, "")
	st.AutoReact.LLMEnabled = true
	repo.seed(st)

	if err := svc.OnMessage(context.Background(), -1, 100, "great news everyone"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if len(tg.reactions) != 1 || tg.reactions[0].emoji != "🔥" {
		t.Fatalf("unexpected reactions: %+v", tg.reactions)
	}

	// Second message inside the 30s cooldown must not hit the LLM again.
	if err := svc.OnMessage(context.Background(), -1, 101, "more news"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if llm.pickCalls != 1 {
		t.Fatalf("want cooldown to suppress LLM, pickCalls=%d", llm.pickCalls)
	}

	// After the cooldown the LLM is consulted again.
	svc.now = func() time.Time { return fixedNow.Add(31 * time.Second) }
	if err := svc.OnMessage(context.Background(), -1, 102, "even more"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if llm.pickCalls != 2 {
		t.Fatalf("want LLM consulted after cooldown, pickCalls=%d", llm.pickCalls)
	}
}

func TestOnMessageLLMUnavailableOrDeclines(t *testing.T) {
	llm := &fakeLLM{available: false}
	svc, repo, tg := setupReaction(llm)
	st := chat.Default(-1, "")
	st.AutoReact.LLMEnabled = true
	repo.seed(st)

	if err := svc.OnMessage(context.Background(), -1, 100, "hi"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if llm.pickCalls != 0 || len(tg.reactions) != 0 {
		t.Fatalf("unavailable LLM must be skipped")
	}

	llm.available = true
	llm.reactionOK = false // LLM decides the message deserves no reaction
	if err := svc.OnMessage(context.Background(), -1, 100, "hi"); err != nil {
		t.Fatalf("OnMessage: %v", err)
	}
	if llm.pickCalls != 1 || len(tg.reactions) != 0 {
		t.Fatalf("declined reaction must not call telegram")
	}
}

func TestReactionRuleManagement(t *testing.T) {
	svc, repo, tg := setupReaction(&fakeLLM{})
	tg.setAdmin(-1, 7, true)
	ctx := context.Background()

	if err := svc.AddKeywordRule(ctx, -1, 8, "nice", "👍"); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	if err := svc.AddKeywordRule(ctx, -1, 7, "", "👍"); err == nil {
		t.Fatalf("want validation error for empty keyword")
	}
	if err := svc.AddKeywordRule(ctx, -1, 7, "nice", "🚫not-allowed"); err == nil {
		t.Fatalf("want validation error for disallowed emoji")
	}
	if err := svc.AddKeywordRule(ctx, -1, 7, "nice", "👍"); err != nil {
		t.Fatalf("AddKeywordRule: %v", err)
	}
	if err := svc.AddKeywordRule(ctx, -1, 7, "nice", "🔥"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}
	if err := svc.AddRegexRule(ctx, -1, 7, "(", "👍"); err == nil {
		t.Fatalf("want validation error for bad regex")
	}
	if err := svc.RemoveRule(ctx, -1, 7, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := svc.RemoveRule(ctx, -1, 7, "nice"); err != nil {
		t.Fatalf("RemoveRule: %v", err)
	}
	if err := svc.SetLLMEnabled(ctx, -1, 7, true); err != nil {
		t.Fatalf("SetLLMEnabled: %v", err)
	}
	st, _ := repo.Get(ctx, -1)
	if len(st.AutoReact.Rules) != 0 || !st.AutoReact.LLMEnabled {
		t.Fatalf("unexpected settings: %+v", st.AutoReact)
	}
}
