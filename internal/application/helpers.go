// Package application contains the use-case services of the bot. Services
// depend only on domain types and ports interfaces; they return structured
// results and sentinel errors, leaving all user-facing wording to the
// interfaces layer.
package application

import (
	"context"
	"errors"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// loadSettings fetches the settings aggregate for a chat, falling back to
// domain defaults when the chat has never been persisted.
func loadSettings(ctx context.Context, repo ports.SettingsRepository, chatID int64) (*chat.Settings, error) {
	st, err := repo.Get(ctx, chatID)
	if errors.Is(err, ports.ErrNotFound) {
		return chat.Default(chatID, ""), nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

// requireAdmin guards an administrative operation.
func requireAdmin(ctx context.Context, tg ports.TelegramGateway, chatID, userID int64) error {
	ok, err := tg.IsAdmin(ctx, chatID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAdmin
	}
	return nil
}

// clockNow is the default wall clock used when a service has no injected clock.
func clockNow() time.Time { return time.Now() }
