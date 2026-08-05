package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

type runnerFixture struct {
	runner   *TaskRunner
	repo     *fakeSettingsRepo
	msglog   *fakeMessageLogRepo
	tasks    *fakeTaskRepo
	captchas *fakeCaptchaRepo
	activity *fakeActivityRepo
	tg       *fakeTelegram
	llm      *fakeLLM
	wl       *fakeWordList
}

func setupRunner() *runnerFixture {
	repo := newFakeSettingsRepo()
	msglog := newFakeMessageLogRepo()
	tasks := newFakeTaskRepo()
	captchas := newFakeCaptchaRepo()
	activity := newFakeActivityRepo()
	tg := newFakeTelegram()
	llm := &fakeLLM{available: true, summaryText: "auto summary"}
	wl := newFakeWordList()

	sum := NewSummaryService(repo, msglog, tasks, tg, llm)
	sum.now = fixedClock
	zom := NewZombieService(repo, activity, tg)
	zom.now = fixedClock
	capSvc := NewCaptchaService(repo, captchas, tg, &fakeRenderer{}, nil)
	capSvc.now = fixedClock
	mod := NewModerationService(repo, tasks, tg, wl)
	mod.now = fixedClock

	return &runnerFixture{
		runner:   NewTaskRunner(tasks, sum, zom, capSvc, mod, tg, repo),
		repo:     repo,
		msglog:   msglog,
		tasks:    tasks,
		captchas: captchas,
		activity: activity,
		tg:       tg,
		llm:      llm,
		wl:       wl,
	}
}

func TestRunnerSweepsExpiredCaptchas(t *testing.T) {
	f := setupRunner()
	_ = f.captchas.Save(context.Background(), &moderation.Session{
		ChatID:    -1,
		UserID:    9,
		Challenge: moderation.Challenge{},
		ExpiresAt: fixedNow.Add(-time.Minute),
	})

	report, err := f.runner.Run(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.ExpiredCaptchas != 1 {
		t.Fatalf("want 1 expired captcha, got %+v", report)
	}
	if len(f.tg.bans) != 1 || f.tg.bans[0].userID != 9 {
		t.Fatalf("want user 9 kicked, got %+v", f.tg.bans)
	}
}

func TestRunnerAutoSummary(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	st := chat.Default(-1, "")
	st.Summary.IntervalHours = 6
	f.repo.seed(st)
	for i := 1; i <= 3; i++ {
		_ = f.msglog.Append(ctx, -1, summary.Message{MessageID: i, Text: "t", At: fixedNow.Add(-time.Hour)})
	}
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindAutoSummary,
		ChatID:        -1,
		IntervalHours: 6,
		NextRunAt:     fixedNow.Add(-time.Minute), // due
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.SummariesSent != 1 || len(report.Errors) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(f.tg.texts) != 1 || f.tg.texts[0].text != "auto summary" {
		t.Fatalf("want summary sent, got %+v", f.tg.texts)
	}
	// Task rescheduled one interval into the future.
	list, _ := f.tasks.List(ctx)
	if len(list) != 1 || !list[0].NextRunAt.Equal(fixedNow.Add(6*time.Hour)) {
		t.Fatalf("want rescheduled task, got %+v", list)
	}
}

func TestRunnerAutoSummaryFailureStillReschedules(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	// No messages at all -> summarize fails with ErrTooFewMessages.
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindAutoSummary,
		ChatID:        -1,
		IntervalHours: 6,
		NextRunAt:     fixedNow.Add(-time.Minute),
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.SummariesSent != 0 || len(report.Errors) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if len(f.tg.texts) != 0 {
		t.Fatalf("no summary should be sent")
	}
	list, _ := f.tasks.List(ctx)
	if !list[0].NextRunAt.After(fixedNow) {
		t.Fatalf("failing task must still be rescheduled")
	}
}

func TestRunnerZombieClean(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	st := chat.Default(-1, "")
	st.Zombie.InactiveDays = 30
	f.repo.seed(st)
	_ = f.activity.Touch(ctx, -2, 5, fixedNow.Add(-90*24*time.Hour)) // zombie
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindZombieClean,
		ChatID:        -2,
		IntervalHours: 24,
		NextRunAt:     fixedNow.Add(-time.Minute),
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.ZombiesKicked != 1 {
		t.Fatalf("want 1 zombie kicked, got %+v", report)
	}
	if len(f.tg.bans) != 1 || f.tg.bans[0].userID != 5 {
		t.Fatalf("want user 5 kicked, got %+v", f.tg.bans)
	}
}

func TestRunnerSkipsFutureTasks(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindZombieClean,
		ChatID:        -3,
		IntervalHours: 24,
		NextRunAt:     fixedNow.Add(time.Hour), // not due
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.ZombiesKicked != 0 || len(report.Errors) != 0 {
		t.Fatalf("unexpected report: %+v", report)
	}
	list, _ := f.tasks.List(ctx)
	if !list[0].NextRunAt.Equal(fixedNow.Add(time.Hour)) {
		t.Fatalf("non-due task must not be rescheduled, got %v", list[0].NextRunAt)
	}
}

func TestRunnerFilterRefresh(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	st := chat.Default(-1, "")
	st.Filter.Sources = []string{"https://x/list.txt"}
	f.repo.seed(st)
	f.wl.rules["https://x/list.txt"] = wordRules("https://x/list.txt", "spam", "scam")
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindFilterRefresh,
		ChatID:        -1,
		IntervalHours: 24,
		NextRunAt:     fixedNow.Add(-time.Minute), // due
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", report.Errors)
	}
	got, _ := f.repo.Get(ctx, -1)
	if len(got.Filter.Rules) != 2 {
		t.Fatalf("want 2 imported rules, got %+v", got.Filter.Rules)
	}
	list, _ := f.tasks.List(ctx)
	if len(list) != 1 || !list[0].NextRunAt.Equal(fixedNow.Add(24*time.Hour)) {
		t.Fatalf("want rescheduled task, got %+v", list)
	}
}

func TestRunnerFilterRefreshFailureStillReschedules(t *testing.T) {
	f := setupRunner()
	ctx := context.Background()
	st := chat.Default(-1, "")
	st.Filter.Sources = []string{"https://x/list.txt"}
	f.repo.seed(st)
	f.wl.errs["https://x/list.txt"] = errors.New("boom")
	_ = f.tasks.Save(ctx, schedule.Task{
		Kind:          schedule.KindFilterRefresh,
		ChatID:        -1,
		IntervalHours: 24,
		NextRunAt:     fixedNow.Add(-time.Minute),
	})

	report, err := f.runner.Run(ctx, fixedNow)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The failed source is recorded inside RefreshResult, not as a sweep error.
	if len(report.Errors) != 0 {
		t.Fatalf("unexpected errors: %+v", report.Errors)
	}
	list, _ := f.tasks.List(ctx)
	if !list[0].NextRunAt.After(fixedNow) {
		t.Fatalf("failing task must still be rescheduled")
	}
}
