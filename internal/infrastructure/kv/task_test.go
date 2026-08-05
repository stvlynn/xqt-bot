package kv

import (
	"context"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
)

func TestTaskKey(t *testing.T) {
	if got := TaskKey(schedule.KindAutoSummary, 12); got != "task:auto_summary:12" {
		t.Fatalf("TaskKey = %q", got)
	}
}

func TestTaskSaveListDelete(t *testing.T) {
	repo := NewTaskRepository(NewMemoryStore())
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	tasks := []schedule.Task{
		schedule.NewTask(schedule.KindAutoSummary, 1, 6, now),
		schedule.NewTask(schedule.KindZombieClean, 1, 24, now),
		schedule.NewTask(schedule.KindAutoSummary, 2, 6, now),
	}
	for _, task := range tasks {
		if err := repo.Save(ctx, task); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
	got, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("List returned %d tasks, want 3", len(got))
	}
	byKey := make(map[string]schedule.Task)
	for _, task := range got {
		byKey[TaskKey(task.Kind, task.ChatID)] = task
	}
	for _, want := range tasks {
		stored, ok := byKey[TaskKey(want.Kind, want.ChatID)]
		if !ok {
			t.Errorf("missing task %s/%d", want.Kind, want.ChatID)
			continue
		}
		if stored.IntervalHours != want.IntervalHours || !stored.NextRunAt.Equal(want.NextRunAt) {
			t.Errorf("round trip mismatch: got %+v want %+v", stored, want)
		}
	}
	if err := repo.Delete(ctx, schedule.KindAutoSummary, 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = repo.List(ctx)
	if len(got) != 2 {
		t.Errorf("List after delete returned %d tasks, want 2", len(got))
	}
	// Deleting a missing task must not fail.
	if err := repo.Delete(ctx, schedule.KindAutoSummary, 1); err != nil {
		t.Errorf("Delete missing task: %v", err)
	}
}

func TestTaskListEmpty(t *testing.T) {
	repo := NewTaskRepository(NewMemoryStore())
	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no tasks, got %v", got)
	}
}
