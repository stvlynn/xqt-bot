package kv

import (
	"context"
	"testing"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

func TestSettingsKey(t *testing.T) {
	if got := SettingsKey(-100123); got != "settings:-100123" {
		t.Fatalf("SettingsKey = %q", got)
	}
}

func TestSettingsGetCreatesAndPersistsDefault(t *testing.T) {
	store := NewMemoryStore()
	repo := NewSettingsRepository(store)
	ctx := context.Background()

	s, err := repo.GetWithDefault(ctx, 42, "Test Group")
	if err != nil {
		t.Fatalf("GetWithDefault: %v", err)
	}
	if s.ChatID != 42 || s.Title != "Test Group" {
		t.Errorf("default settings mismatch: %+v", s)
	}
	// First contact must materialize the record in the store.
	if _, err := store.Get(ctx, SettingsKey(42)); err != nil {
		t.Fatalf("default was not persisted: %v", err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	repo := NewSettingsRepository(store)
	ctx := context.Background()

	s := chat.Default(7, "grp")
	s.Welcome.Enabled = true
	s.Welcome.Text = "hi {name}"
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Get(ctx, 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Welcome.Enabled || got.Welcome.Text != "hi {name}" {
		t.Errorf("round trip mismatch: %+v", got.Welcome)
	}
}

func TestSettingsGetUnknownChatBuildsEmptyTitleDefault(t *testing.T) {
	repo := NewSettingsRepository(NewMemoryStore())
	s, err := repo.Get(context.Background(), 99)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.ChatID != 99 || s.Title != "" {
		t.Errorf("unexpected default: %+v", s)
	}
}

func TestMemoryStoreNotFoundAndListKeys(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	if _, err := store.Get(ctx, "missing"); err == nil {
		t.Fatal("expected error for missing key")
	}
	_ = store.Put(ctx, "a:2", []byte("x"), 0)
	_ = store.Put(ctx, "a:1", []byte("x"), 0)
	_ = store.Put(ctx, "b:1", []byte("x"), 0)
	keys, err := store.ListKeys(ctx, "a:")
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(keys) != 2 || keys[0] != "a:1" || keys[1] != "a:2" {
		t.Errorf("ListKeys = %v", keys)
	}
	_ = store.Delete(ctx, "a:1")
	if _, err := store.Get(ctx, "a:1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestMemoryStoreCopyIsolation(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	val := []byte("abc")
	_ = store.Put(ctx, "k", val, 0)
	val[0] = 'X' // mutating caller buffer must not affect the store
	got, _ := store.Get(ctx, "k")
	if string(got) != "abc" {
		t.Fatalf("store not isolated from caller buffer: %q", got)
	}
	got[0] = 'Y' // mutating the returned slice must not affect the store
	got2, _ := store.Get(ctx, "k")
	if string(got2) != "abc" {
		t.Fatalf("store not isolated from returned buffer: %q", got2)
	}
}
