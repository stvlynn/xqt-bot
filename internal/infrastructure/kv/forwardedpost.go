package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// ForwardedPostTTLSeconds is how long a post mapping is kept: comment
// buttons can no longer be updated once it expires.
const ForwardedPostTTLSeconds = 7 * 24 * 3600

// ForwardedPostRepository is a Store-backed ports.ForwardedPostRepository.
type ForwardedPostRepository struct {
	store Store
}

// NewForwardedPostRepository creates the repository over the given Store.
func NewForwardedPostRepository(store Store) *ForwardedPostRepository {
	return &ForwardedPostRepository{store: store}
}

// ForwardedPostPrefix selects all forwarded-post keys.
const ForwardedPostPrefix = "chanpost:"

// ForwardedPostKey builds the storage key for one channel post's mapping.
func ForwardedPostKey(channelID int64, postID int) string {
	return fmt.Sprintf("%s%d:%d", ForwardedPostPrefix, channelID, postID)
}

// Save implements ports.ForwardedPostRepository.
func (r *ForwardedPostRepository) Save(ctx context.Context, p channelpost.ForwardedPost) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("chanpost repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, ForwardedPostKey(p.ChannelID, p.PostID), raw, ForwardedPostTTLSeconds); err != nil {
		return fmt.Errorf("chanpost repo: put: %w", err)
	}
	return nil
}

// Get implements ports.ForwardedPostRepository.
func (r *ForwardedPostRepository) Get(ctx context.Context, channelID int64, postID int) (*channelpost.ForwardedPost, error) {
	raw, err := r.store.Get(ctx, ForwardedPostKey(channelID, postID))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("chanpost repo: get %d/%d: %w", channelID, postID, err)
	}
	var p channelpost.ForwardedPost
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("chanpost repo: decode %d/%d: %w", channelID, postID, err)
	}
	return &p, nil
}
