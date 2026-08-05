//go:build js && wasm

package main

import (
	"context"

	"github.com/syumai/workers"
	"github.com/syumai/workers/cloudflare/cron"

	ifcron "github.com/stvlynn/xqt-bot/internal/interfaces/cron"
)

func main() {
	handler, runner, err := setup()
	if err != nil {
		panic(err)
	}
	// Non-blocking: the scheduled task shares the isolate with the HTTP
	// server below (wrangler.toml registers the cron trigger).
	cron.ScheduleTaskNonBlock(func(ctx context.Context) error {
		return ifcron.RunOnce(ctx, runner)
	})
	workers.Serve(handler)
}
