package kv

import (
	"context"
	"testing"
	"time"
)

func TestActivityKey(t *testing.T) {
	if got := ActivityKey(-1005); got != "activity:-1005" {
		t.Fatalf("ActivityKey = %q", got)
	}
}

func TestActivityTouchAndLastSeen(t *testing.T) {
	repo := NewActivityRepository(NewMemoryStore())
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	if err := repo.Touch(ctx, 1, 100, now); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := repo.Touch(ctx, 1, 200, now.Add(time.Minute)); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	seen, err := repo.LastSeen(ctx, 1)
	if err != nil {
		t.Fatalf("LastSeen: %v", err)
	}
	if len(seen) != 2 || !seen[100].Equal(now) {
		t.Errorf("LastSeen = %v", seen)
	}
}

func TestActivityRemoveAndEmptyCleanup(t *testing.T) {
	repo := NewActivityRepository(NewMemoryStore())
	store := repo.store
	ctx := context.Background()
	if err := repo.Remove(ctx, 1, 999); err != nil { // untracked: no-op
		t.Fatalf("Remove untracked: %v", err)
	}
	_ = repo.Touch(ctx, 1, 100, time.Now())
	if err := repo.Remove(ctx, 1, 100); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := store.Get(ctx, ActivityKey(1)); err == nil {
		t.Fatal("expected key deleted once map is empty")
	}
}

func TestActivityLastSeenUnknownChatIsEmpty(t *testing.T) {
	repo := NewActivityRepository(NewMemoryStore())
	seen, err := repo.LastSeen(context.Background(), 42)
	if err != nil {
		t.Fatalf("LastSeen: %v", err)
	}
	if len(seen) != 0 {
		t.Errorf("expected empty map, got %v", seen)
	}
}

func TestPruneActivityEvictsOldest(t *testing.T) {
	seen := make(map[int64]time.Time)
	base := time.Now()
	for i := 0; i < maxActivityEntries+50; i++ {
		seen[int64(i)] = base.Add(time.Duration(i) * time.Second)
	}
	seen = pruneActivity(seen, maxActivityEntries)
	if len(seen) != maxActivityEntries {
		t.Fatalf("len = %d, want %d", len(seen), maxActivityEntries)
	}
	if _, ok := seen[0]; ok {
		t.Error("oldest entry should have been evicted")
	}
	if _, ok := seen[int64(maxActivityEntries+49)]; !ok {
		t.Error("newest entry should have been kept")
	}
}

func TestActivityTouchPrunesThroughStore(t *testing.T) {
	repo := NewActivityRepository(NewMemoryStore())
	ctx := context.Background()
	base := time.Now()
	for i := 0; i < maxActivityEntries+1; i++ {
		if err := repo.Touch(ctx, 7, int64(i), base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("Touch: %v", err)
		}
	}
	seen, err := repo.LastSeen(ctx, 7)
	if err != nil {
		t.Fatalf("LastSeen: %v", err)
	}
	if len(seen) != maxActivityEntries {
		t.Errorf("len = %d, want %d", len(seen), maxActivityEntries)
	}
}
