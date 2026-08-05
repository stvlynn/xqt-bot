package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// SettingsRepository is a Store-backed ports.SettingsRepository.
type SettingsRepository struct {
	store Store
}

// NewSettingsRepository creates the repository over the given Store.
func NewSettingsRepository(store Store) *SettingsRepository {
	return &SettingsRepository{store: store}
}

// SettingsKey builds the storage key for one chat's settings.
func SettingsKey(chatID int64) string {
	return fmt.Sprintf("settings:%d", chatID)
}

// Get loads the settings aggregate. On first contact with a chat it
// materializes chat.Default (with an empty title) and persists it, so a
// later Save always updates an existing record.
func (r *SettingsRepository) Get(ctx context.Context, chatID int64) (*chat.Settings, error) {
	return r.GetWithDefault(ctx, chatID, "")
}

// GetWithDefault is Get with the chat title known by the caller (e.g. from
// the update that triggered the lookup), used when creating the default.
func (r *SettingsRepository) GetWithDefault(ctx context.Context, chatID int64, title string) (*chat.Settings, error) {
	raw, err := r.store.Get(ctx, SettingsKey(chatID))
	switch {
	case err == nil:
		var s chat.Settings
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("settings repo: decode %d: %w", chatID, err)
		}
		return &s, nil
	case errors.Is(err, ports.ErrNotFound):
		s := chat.Default(chatID, title)
		if err := r.Save(ctx, s); err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, fmt.Errorf("settings repo: get %d: %w", chatID, err)
	}
}

// Save persists the aggregate.
func (r *SettingsRepository) Save(ctx context.Context, settings *chat.Settings) error {
	raw, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("settings repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, SettingsKey(settings.ChatID), raw, 0); err != nil {
		return fmt.Errorf("settings repo: put: %w", err)
	}
	return nil
}
