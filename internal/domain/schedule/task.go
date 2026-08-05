// Package schedule defines per-chat recurring tasks evaluated by the
// worker's cron handler.
package schedule

import "time"

// Kind identifies a recurring task type.
type Kind string

const (
	KindAutoSummary Kind = "auto_summary" // periodic LLM chat summary
	KindZombieClean Kind = "zombie_clean" // kick members inactive beyond the threshold
)

// Task is one recurring job for one chat. NextRunAt is recomputed after
// every execution from IntervalHours.
type Task struct {
	Kind          Kind      `json:"kind"`
	ChatID        int64     `json:"chat_id"`
	IntervalHours int       `json:"interval_hours"`
	NextRunAt     time.Time `json:"next_run_at"`
}

// NewTask builds a task whose first run happens after one interval.
func NewTask(kind Kind, chatID int64, intervalHours int, now time.Time) Task {
	if intervalHours < 1 {
		intervalHours = 1
	}
	return Task{
		Kind:          kind,
		ChatID:        chatID,
		IntervalHours: intervalHours,
		NextRunAt:     now.Add(time.Duration(intervalHours) * time.Hour),
	}
}

// Due reports whether the task should run now.
func (t Task) Due(now time.Time) bool {
	return !now.Before(t.NextRunAt)
}

// Rescheduled returns the task with its next run one interval after `now`.
func (t Task) Rescheduled(now time.Time) Task {
	t.NextRunAt = now.Add(time.Duration(t.IntervalHours) * time.Hour)
	return t
}
