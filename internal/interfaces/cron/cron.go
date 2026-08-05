// Package cron adapts Cloudflare's scheduled trigger to the application's
// TaskRunner. It contains no js/wasm imports so the scheduling glue in main
// can wrap it for both targets.
package cron

import (
	"context"
	"log"
	"time"

	"github.com/stvlynn/xqt-bot/internal/application"
)

// RunOnce executes one cron sweep and logs the resulting report. Failures of
// individual tasks are already collected in the report; RunOnce only returns
// an error when the sweep itself cannot run.
func RunOnce(ctx context.Context, runner *application.TaskRunner) error {
	report, err := runner.Run(ctx, time.Now())
	if err != nil {
		return err
	}
	log.Printf("cron sweep: expired captchas=%d summaries sent=%d zombies kicked=%d task errors=%d",
		report.ExpiredCaptchas, report.SummariesSent, report.ZombiesKicked, len(report.Errors))
	for _, e := range report.Errors {
		log.Printf("cron task error: %v", e)
	}
	return nil
}
