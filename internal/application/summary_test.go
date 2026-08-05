package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

func setupSummary(llm *fakeLLM) (*SummaryService, *fakeSettingsRepo, *fakeMessageLogRepo, *fakeTaskRepo, *fakeTelegram) {
	repo := newFakeSettingsRepo()
	msglog := newFakeMessageLogRepo()
	tasks := newFakeTaskRepo()
	tg := newFakeTelegram()
	svc := NewSummaryService(repo, msglog, tasks, tg, llm)
	svc.now = fixedClock
	return svc, repo, msglog, tasks, tg
}

func seedMessages(t *testing.T, msglog *fakeMessageLogRepo, chatID int64, msgs ...summary.Message) {
	t.Helper()
	for _, m := range msgs {
		if err := msglog.Append(context.Background(), chatID, m); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func recentMsg(id int, ago time.Duration) summary.Message {
	return summary.Message{MessageID: id, UserID: 1, UserName: "u", Text: "text", At: fixedNow.Add(-ago)}
}

func TestRecordMessage(t *testing.T) {
	svc, _, msglog, _, _ := setupSummary(&fakeLLM{})
	if err := svc.RecordMessage(context.Background(), -1, recentMsg(1, time.Minute)); err != nil {
		t.Fatalf("RecordMessage: %v", err)
	}
	msgs, _ := msglog.Recent(context.Background(), -1)
	if len(msgs) != 1 {
		t.Fatalf("want 1 message, got %d", len(msgs))
	}
}

func TestSummarizeNowRequiresAdmin(t *testing.T) {
	svc, _, _, _, _ := setupSummary(&fakeLLM{available: true})
	if _, err := svc.SummarizeNow(context.Background(), -1, 8, 24); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
}

func TestSummarizeNowTooFewMessages(t *testing.T) {
	svc, _, msglog, _, tg := setupSummary(&fakeLLM{available: true})
	tg.setAdmin(-1, 7, true)
	seedMessages(t, msglog, -1, recentMsg(1, time.Hour), recentMsg(2, time.Hour))

	if _, err := svc.SummarizeNow(context.Background(), -1, 7, 24); !errors.Is(err, ErrTooFewMessages) {
		t.Fatalf("want ErrTooFewMessages, got %v", err)
	}
}

func TestSummarizeNowLLMNotConfigured(t *testing.T) {
	svc, _, msglog, _, tg := setupSummary(&fakeLLM{available: false})
	tg.setAdmin(-1, 7, true)
	seedMessages(t, msglog, -1, recentMsg(1, time.Hour), recentMsg(2, time.Hour), recentMsg(3, time.Hour))

	if _, err := svc.SummarizeNow(context.Background(), -1, 7, 24); !errors.Is(err, ErrLLMNotConfigured) {
		t.Fatalf("want ErrLLMNotConfigured, got %v", err)
	}
}

func TestSummarizeNowFiltersWindow(t *testing.T) {
	llm := &fakeLLM{available: true, summaryText: "summary text"}
	svc, _, msglog, _, tg := setupSummary(llm)
	tg.setAdmin(-1, 7, true)
	seedMessages(t, msglog, -1,
		recentMsg(1, 48*time.Hour), // outside the 24h window
		recentMsg(2, time.Hour),
		recentMsg(3, 2*time.Hour),
		recentMsg(4, 3*time.Hour),
	)

	res, err := svc.SummarizeNow(context.Background(), -1, 7, 24)
	if err != nil {
		t.Fatalf("SummarizeNow: %v", err)
	}
	if res.Text != "summary text" || res.MessageCount != 3 || res.Hours != 24 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestSummarizeNowDefaultHours(t *testing.T) {
	llm := &fakeLLM{available: true, summaryText: "s"}
	svc, _, msglog, _, tg := setupSummary(llm)
	tg.setAdmin(-1, 7, true)
	seedMessages(t, msglog, -1, recentMsg(1, time.Hour), recentMsg(2, time.Hour), recentMsg(3, time.Hour))

	res, err := svc.SummarizeNow(context.Background(), -1, 7, 0) // 0 = default
	if err != nil {
		t.Fatalf("SummarizeNow: %v", err)
	}
	if res.Hours != 24 {
		t.Fatalf("want default 24 hours, got %d", res.Hours)
	}
}

func TestSetAutoSummary(t *testing.T) {
	svc, repo, _, tasks, tg := setupSummary(&fakeLLM{})
	tg.setAdmin(-1, 7, true)
	ctx := context.Background()

	if err := svc.SetAutoSummary(ctx, -1, 8, 6); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}

	if err := svc.SetAutoSummary(ctx, -1, 7, 6); err != nil {
		t.Fatalf("SetAutoSummary enable: %v", err)
	}
	st, _ := repo.Get(ctx, -1)
	if !st.Summary.AutoEnabled || st.Summary.IntervalHours != 6 {
		t.Fatalf("unexpected settings: %+v", st.Summary)
	}
	list, _ := tasks.List(ctx)
	if len(list) != 1 || list[0].Kind != schedule.KindAutoSummary || list[0].IntervalHours != 6 {
		t.Fatalf("unexpected tasks: %+v", list)
	}
	if want := fixedNow.Add(6 * time.Hour); !list[0].NextRunAt.Equal(want) {
		t.Fatalf("want next run %v, got %v", want, list[0].NextRunAt)
	}

	// Disabling removes the task and clears the flag.
	if err := svc.SetAutoSummary(ctx, -1, 7, 0); err != nil {
		t.Fatalf("SetAutoSummary disable: %v", err)
	}
	st, _ = repo.Get(ctx, -1)
	list, _ = tasks.List(ctx)
	if st.Summary.AutoEnabled || len(list) != 0 {
		t.Fatalf("want disabled and no tasks, got %+v / %+v", st.Summary, list)
	}
}
