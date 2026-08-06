package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// ChannelBindingRepository is a Store-backed ports.ChannelBindingRepository.
type ChannelBindingRepository struct {
	store Store
}

// NewChannelBindingRepository creates the repository over the given Store.
func NewChannelBindingRepository(store Store) *ChannelBindingRepository {
	return &ChannelBindingRepository{store: store}
}

// ChannelBindingPrefix selects all channel-binding keys.
const ChannelBindingPrefix = "chanbind:"

// ChannelBindingKey builds the storage key for one channel's binding.
func ChannelBindingKey(channelID int64) string {
	return fmt.Sprintf("%s%d", ChannelBindingPrefix, channelID)
}

// Set records that the channel forwards into the given group. Bindings have
// no storage TTL: they live until an admin unbinds.
func (r *ChannelBindingRepository) Set(ctx context.Context, channelID, groupID int64) error {
	raw, err := json.Marshal(groupID)
	if err != nil {
		return fmt.Errorf("chanbind repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, ChannelBindingKey(channelID), raw, 0); err != nil {
		return fmt.Errorf("chanbind repo: put: %w", err)
	}
	return nil
}

// GetByChannel implements ports.ChannelBindingRepository.
func (r *ChannelBindingRepository) GetByChannel(ctx context.Context, channelID int64) (int64, error) {
	raw, err := r.store.Get(ctx, ChannelBindingKey(channelID))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return 0, ports.ErrNotFound
		}
		return 0, fmt.Errorf("chanbind repo: get %d: %w", channelID, err)
	}
	var groupID int64
	if err := json.Unmarshal(raw, &groupID); err != nil {
		return 0, fmt.Errorf("chanbind repo: decode %d: %w", channelID, err)
	}
	return groupID, nil
}

// Delete implements ports.ChannelBindingRepository.
func (r *ChannelBindingRepository) Delete(ctx context.Context, channelID int64) error {
	if err := r.store.Delete(ctx, ChannelBindingKey(channelID)); err != nil {
		return fmt.Errorf("chanbind repo: delete: %w", err)
	}
	return nil
}
