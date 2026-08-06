package kv

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

func TestChannelBindingRepository(t *testing.T) {
	r := NewChannelBindingRepository(NewMemoryStore())
	ctx := context.Background()

	if _, err := r.GetByChannel(ctx, 1); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByChannel on empty = %v, want ErrNotFound", err)
	}
	if err := r.Set(ctx, 1, 100); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetByChannel(ctx, 1)
	if err != nil || got != 100 {
		t.Fatalf("GetByChannel = %d, %v; want 100, nil", got, err)
	}
	if err := r.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetByChannel(ctx, 1); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("GetByChannel after delete = %v, want ErrNotFound", err)
	}
}

func TestForwardedPostRepository(t *testing.T) {
	r := NewForwardedPostRepository(NewMemoryStore())
	ctx := context.Background()

	if _, err := r.Get(ctx, 1, 2); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get on empty = %v, want ErrNotFound", err)
	}
	p := channelpost.ForwardedPost{ChannelID: 1, PostID: 2, GroupID: 100, GroupMessageID: 55}
	if err := r.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if *got != p {
		t.Fatalf("Get = %+v, want %+v", *got, p)
	}
}

func TestCommentLogRepository(t *testing.T) {
	r := NewCommentLogRepository(NewMemoryStore())
	ctx := context.Background()

	if got, err := r.List(ctx, 1, 2); err != nil || len(got) != 0 {
		t.Fatalf("List on empty = %v, %v; want empty, nil", got, err)
	}
	for i := 0; i < channelpost.CommentPreviewCapacity+1; i++ {
		c := channelpost.Comment{MessageID: i + 1, Author: "a", Text: "x", At: time.Now()}
		if err := r.Append(ctx, 1, 2, c); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.List(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != channelpost.CommentPreviewCapacity {
		t.Fatalf("len = %d, want %d", len(got), channelpost.CommentPreviewCapacity)
	}
	if got[0].MessageID != 2 {
		t.Fatalf("oldest kept MessageID = %d, want 2", got[0].MessageID)
	}
}
