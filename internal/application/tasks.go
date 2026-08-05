package application

import (
	"context"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
)

// RunReport summarizes one cron sweep.
type RunReport struct {
	ExpiredCaptchas int
	SummariesSent   int
	ZombiesKicked   int
	Errors          []error
}

// TaskRunner executes the recurring per-chat tasks due at each cron tick.
// Individual task failures are collected in the report and never abort the
// sweep; every executed task is still rescheduled.
type TaskRunner struct {
	tasks    ports.TaskRepository
	sum      *SummaryService
	zom      *ZombieService
	cap      *CaptchaService
	mod      *ModerationService
	tg       ports.TelegramGateway
	settings ports.SettingsRepository
}

// NewTaskRunner builds the runner.
func NewTaskRunner(tasks ports.TaskRepository, sum *SummaryService, zom *ZombieService, cap *CaptchaService, mod *ModerationService, tg ports.TelegramGateway, settings ports.SettingsRepository) *TaskRunner {
	return &TaskRunner{
		tasks:    tasks,
		sum:      sum,
		zom:      zom,
		cap:      cap,
		mod:      mod,
		tg:       tg,
		settings: settings,
	}
}

// Run sweeps expired captchas and executes every due task once.
func (r *TaskRunner) Run(ctx context.Context, now time.Time) (*RunReport, error) {
	report := &RunReport{}

	expired, err := r.cap.SweepExpired(ctx, now)
	if err != nil {
		report.Errors = append(report.Errors, err)
	} else {
		report.ExpiredCaptchas = expired
	}

	tasks, err := r.tasks.List(ctx)
	if err != nil {
		report.Errors = append(report.Errors, err)
		return report, nil
	}

	for _, t := range tasks {
		if !t.Due(now) {
			continue
		}
		switch t.Kind {
		case schedule.KindAutoSummary:
			r.runAutoSummary(ctx, t, report)
		case schedule.KindZombieClean:
			r.runZombieClean(ctx, t, report)
		case schedule.KindFilterRefresh:
			r.runFilterRefresh(ctx, t, report)
		}
		if err := r.tasks.Save(ctx, t.Rescheduled(now)); err != nil {
			report.Errors = append(report.Errors, err)
		}
	}
	return report, nil
}

// runAutoSummary summarizes the chat's recent messages and posts the result.
func (r *TaskRunner) runAutoSummary(ctx context.Context, t schedule.Task, report *RunReport) {
	hours := t.IntervalHours
	if st, err := loadSettings(ctx, r.settings, t.ChatID); err == nil && st.Summary.IntervalHours > 0 {
		hours = st.Summary.IntervalHours
	}
	res, err := r.sum.summarizeChat(ctx, t.ChatID, hours)
	if err != nil {
		report.Errors = append(report.Errors, err)
		return
	}
	if _, err := r.tg.SendText(ctx, t.ChatID, res.Text, nil); err != nil {
		report.Errors = append(report.Errors, err)
		return
	}
	report.SummariesSent++
}

// runZombieClean kicks the chat's zombie members (system task, no admin check).
func (r *TaskRunner) runZombieClean(ctx context.Context, t schedule.Task, report *RunReport) {
	res, err := r.zom.clean(ctx, t.ChatID)
	if err != nil {
		report.Errors = append(report.Errors, err)
		return
	}
	report.ZombiesKicked += len(res.Kicked)
}

// runFilterRefresh re-imports the chat's remote word lists (system task, no
// admin check).
func (r *TaskRunner) runFilterRefresh(ctx context.Context, t schedule.Task, report *RunReport) {
	if _, err := r.mod.RefreshAllWordLists(ctx, t.ChatID); err != nil {
		report.Errors = append(report.Errors, err)
	}
}
