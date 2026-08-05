package kv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
)

// TaskRepository is a Store-backed ports.TaskRepository.
type TaskRepository struct {
	store Store
}

// NewTaskRepository creates the repository over the given Store.
func NewTaskRepository(store Store) *TaskRepository {
	return &TaskRepository{store: store}
}

// TaskPrefix selects all task keys.
const TaskPrefix = "task:"

// TaskKey builds the storage key for one recurring task.
func TaskKey(kind schedule.Kind, chatID int64) string {
	return fmt.Sprintf("%s%s:%d", TaskPrefix, kind, chatID)
}

// List returns every task across all chats. Corrupt entries are skipped
// rather than failing the whole cron sweep.
func (r *TaskRepository) List(ctx context.Context) ([]schedule.Task, error) {
	keys, err := r.store.ListKeys(ctx, TaskPrefix)
	if err != nil {
		return nil, fmt.Errorf("task repo: list: %w", err)
	}
	tasks := make([]schedule.Task, 0, len(keys))
	for _, key := range keys {
		raw, err := r.store.Get(ctx, key)
		if err != nil {
			continue
		}
		var t schedule.Task
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// Save persists one task.
func (r *TaskRepository) Save(ctx context.Context, t schedule.Task) error {
	raw, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("task repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, TaskKey(t.Kind, t.ChatID), raw, 0); err != nil {
		return fmt.Errorf("task repo: put: %w", err)
	}
	return nil
}

// Delete removes one task. Deleting a missing task is not an error.
func (r *TaskRepository) Delete(ctx context.Context, kind schedule.Kind, chatID int64) error {
	if err := r.store.Delete(ctx, TaskKey(kind, chatID)); err != nil {
		return fmt.Errorf("task repo: delete: %w", err)
	}
	return nil
}
