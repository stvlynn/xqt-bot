package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/reaction"
)

type pipelineFixture struct {
	pipeline *GroupMessagePipeline
	repo     *fakeSettingsRepo
	msglog   *fakeMessageLogRepo
	activity *fakeActivityRepo
	tg       *fakeTelegram
}

func setupPipeline() *pipelineFixture {
	repo := newFakeSettingsRepo()
	msglog := newFakeMessageLogRepo()
	tasks := newFakeTaskRepo()
	activity := newFakeActivityRepo()
	tg := newFakeTelegram()
	llm := &fakeLLM{}

	mod := NewModerationService(repo, tg)
	mod.now = fixedClock
	react := NewReactionService(repo, tg, llm)
	react.now = fixedClock
	sum := NewSummaryService(repo, msglog, tasks, tg, llm)
	sum.now = fixedClock
	zom := NewZombieService(repo, activity, tg)
	zom.now = fixedClock

	p := NewGroupMessagePipeline(mod, react, sum, zom)
	p.now = fixedClock
	return &pipelineFixture{pipeline: p, repo: repo, msglog: msglog, activity: activity, tg: tg}
}

func TestPipelineCleanMessage(t *testing.T) {
	f := setupPipeline()
	st := chat.Default(-1, "")
	st.Filter.Rules = nil
	st.AutoReact.Rules = []reaction.Rule{{Kind: reaction.KindKeyword, Pattern: "nice", Emoji: "👍"}}
	f.repo.seed(st)

	err := f.pipeline.HandleMessage(context.Background(), -1, 42, 100, "alice", "nice work")
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	seen, _ := f.activity.LastSeen(context.Background(), -1)
	if !seen[42].Equal(fixedNow) {
		t.Fatalf("want activity touch")
	}
	msgs, _ := f.msglog.Recent(context.Background(), -1)
	if len(msgs) != 1 || msgs[0].UserName != "alice" || msgs[0].MessageID != 100 {
		t.Fatalf("want message recorded, got %+v", msgs)
	}
	if len(f.tg.reactions) != 1 || f.tg.reactions[0].emoji != "👍" {
		t.Fatalf("want reaction, got %+v", f.tg.reactions)
	}
}

func TestPipelineHitSkipsReaction(t *testing.T) {
	f := setupPipeline()
	st := chat.Default(-1, "")
	st.Filter.Rules = []moderation.FilterRule{{Kind: moderation.RuleWord, Pattern: "spam"}}
	st.AutoReact.Rules = []reaction.Rule{{Kind: reaction.KindKeyword, Pattern: "spam", Emoji: "💩"}}
	f.repo.seed(st)

	err := f.pipeline.HandleMessage(context.Background(), -1, 42, 100, "mallory", "spam spam")
	if err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	if len(f.tg.deleted) != 1 {
		t.Fatalf("want message deleted, got %+v", f.tg.deleted)
	}
	if len(f.tg.reactions) != 0 {
		t.Fatalf("moderated message must not get a reaction")
	}
	// Earlier steps still ran.
	msgs, _ := f.msglog.Recent(context.Background(), -1)
	if len(msgs) != 1 {
		t.Fatalf("want message recorded even when moderated")
	}
}

func TestPipelineAggregatesStepErrors(t *testing.T) {
	f := setupPipeline()
	st := chat.Default(-1, "")
	st.Filter.Rules = nil
	st.AutoReact.Rules = []reaction.Rule{{Kind: reaction.KindKeyword, Pattern: "nice", Emoji: "👍"}}
	f.repo.seed(st)
	f.msglog.appendErr = errors.New("log backend down")

	err := f.pipeline.HandleMessage(context.Background(), -1, 42, 100, "alice", "nice")
	if err == nil {
		t.Fatalf("want aggregated error")
	}
	// Later steps must still have run despite the logging failure.
	if len(f.tg.reactions) != 1 {
		t.Fatalf("want reaction despite logging failure")
	}
	seen, _ := f.activity.LastSeen(context.Background(), -1)
	if len(seen) != 1 {
		t.Fatalf("want activity touch despite logging failure")
	}
}
