package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// MessageLogCapacity is the per-chat ring size persisted to the store.
const MessageLogCapacity = 500

// MessageLogRepository is a Store-backed ports.MessageLogRepository.
type MessageLogRepository struct {
	store Store
}

// NewMessageLogRepository creates the repository over the given Store.
func NewMessageLogRepository(store Store) *MessageLogRepository {
	return &MessageLogRepository{store: store}
}

// MessageLogKey builds the storage key for one chat's message ring.
func MessageLogKey(chatID int64) string {
	return fmt.Sprintf("msglog:%d", chatID)
}

// Append records one message, creating the ring on first use and evicting
// the oldest entry once the ring is full.
func (r *MessageLogRepository) Append(ctx context.Context, chatID int64, m summary.Message) error {
	ring, err := r.load(ctx, chatID)
	if err != nil {
		return err
	}
	ring.Append(m)
	raw, err := json.Marshal(ring)
	if err != nil {
		return fmt.Errorf("msglog repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, MessageLogKey(chatID), raw, 0); err != nil {
		return fmt.Errorf("msglog repo: put: %w", err)
	}
	return nil
}

// Recent returns the recorded messages, oldest first. An unknown chat
// yields an empty slice (no error).
func (r *MessageLogRepository) Recent(ctx context.Context, chatID int64) ([]summary.Message, error) {
	ring, err := r.load(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return ring.Messages, nil
}

// load fetches the ring, transparently creating an empty one on a miss.
func (r *MessageLogRepository) load(ctx context.Context, chatID int64) (*summary.Ring, error) {
	raw, err := r.store.Get(ctx, MessageLogKey(chatID))
	switch {
	case err == nil:
		var ring summary.Ring
		if err := json.Unmarshal(raw, &ring); err != nil {
			return nil, fmt.Errorf("msglog repo: decode %d: %w", chatID, err)
		}
		return &ring, nil
	case errors.Is(err, ports.ErrNotFound):
		return summary.NewRing(MessageLogCapacity), nil
	default:
		return nil, fmt.Errorf("msglog repo: get %d: %w", chatID, err)
	}
}
