package kv

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

func TestMessageLogKey(t *testing.T) {
	if got := MessageLogKey(88); got != "msglog:88" {
		t.Fatalf("MessageLogKey = %q", got)
	}
}

func TestMessageLogAppendCreatesRing(t *testing.T) {
	repo := NewMessageLogRepository(NewMemoryStore())
	ctx := context.Background()
	m := summary.Message{MessageID: 1, UserID: 2, UserName: "ann", Text: "hi", At: time.Now()}
	if err := repo.Append(ctx, 5, m); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := repo.Recent(ctx, 5)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 1 || got[0].Text != "hi" {
		t.Errorf("Recent = %+v", got)
	}
}

func TestMessageLogRecentUnknownChatIsEmpty(t *testing.T) {
	repo := NewMessageLogRepository(NewMemoryStore())
	got, err := repo.Recent(context.Background(), 404)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestMessageLogCapacityTrim(t *testing.T) {
	repo := NewMessageLogRepository(NewMemoryStore())
	ctx := context.Background()
	for i := 1; i <= MessageLogCapacity+10; i++ {
		m := summary.Message{
			MessageID: i,
			UserID:    1,
			UserName:  "u",
			Text:      fmt.Sprintf("m%d", i),
			At:        time.Now(),
		}
		if err := repo.Append(ctx, 9, m); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	got, err := repo.Recent(ctx, 9)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != MessageLogCapacity {
		t.Fatalf("ring size = %d, want %d", len(got), MessageLogCapacity)
	}
	// Oldest entries were evicted; the newest survives last.
	if got[0].MessageID != 11 || got[len(got)-1].MessageID != MessageLogCapacity+10 {
		t.Errorf("eviction order wrong: first=%d last=%d", got[0].MessageID, got[len(got)-1].MessageID)
	}
}
