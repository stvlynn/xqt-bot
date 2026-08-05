package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// maxActivityEntries bounds the per-chat activity map; beyond it the
// oldest-seen members are evicted (zombie cleanup tracks recent activity,
// so dropping the stalest records first is the safe direction).
const maxActivityEntries = 2000

// ActivityRepository is a Store-backed ports.ActivityRepository.
type ActivityRepository struct {
	store Store
}

// NewActivityRepository creates the repository over the given Store.
func NewActivityRepository(store Store) *ActivityRepository {
	return &ActivityRepository{store: store}
}

// ActivityKey builds the storage key for one chat's activity map.
func ActivityKey(chatID int64) string {
	return fmt.Sprintf("activity:%d", chatID)
}

// Touch records activity for one member, evicting the oldest entries when
// the map exceeds maxActivityEntries.
func (r *ActivityRepository) Touch(ctx context.Context, chatID, userID int64, at time.Time) error {
	seen, err := r.load(ctx, chatID)
	if err != nil {
		return err
	}
	seen[userID] = at
	seen = pruneActivity(seen, maxActivityEntries)
	return r.persist(ctx, chatID, seen)
}

// LastSeen returns the full activity map for the chat (empty when unseen).
func (r *ActivityRepository) LastSeen(ctx context.Context, chatID int64) (map[int64]time.Time, error) {
	return r.load(ctx, chatID)
}

// Remove drops one member from the activity map. Removing an untracked
// member is a no-op; the key is deleted once the map is empty.
func (r *ActivityRepository) Remove(ctx context.Context, chatID, userID int64) error {
	seen, err := r.load(ctx, chatID)
	if err != nil {
		return err
	}
	if _, ok := seen[userID]; !ok {
		return nil
	}
	delete(seen, userID)
	return r.persist(ctx, chatID, seen)
}

// load fetches and decodes the activity map, returning an empty map on a
// miss. JSON object keys are strings, so user IDs round-trip via strconv.
func (r *ActivityRepository) load(ctx context.Context, chatID int64) (map[int64]time.Time, error) {
	seen := make(map[int64]time.Time)
	raw, err := r.store.Get(ctx, ActivityKey(chatID))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return seen, nil
		}
		return nil, fmt.Errorf("activity repo: get %d: %w", chatID, err)
	}
	encoded := make(map[string]time.Time)
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, fmt.Errorf("activity repo: decode %d: %w", chatID, err)
	}
	for k, v := range encoded {
		userID, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			continue // ignore corrupt key rather than failing the whole map
		}
		seen[userID] = v
	}
	return seen, nil
}

// persist encodes the map (user IDs as decimal string keys) and stores it,
// deleting the key when nothing is tracked anymore.
func (r *ActivityRepository) persist(ctx context.Context, chatID int64, seen map[int64]time.Time) error {
	key := ActivityKey(chatID)
	if len(seen) == 0 {
		if err := r.store.Delete(ctx, key); err != nil {
			return fmt.Errorf("activity repo: delete: %w", err)
		}
		return nil
	}
	encoded := make(map[string]time.Time, len(seen))
	for userID, at := range seen {
		encoded[strconv.FormatInt(userID, 10)] = at
	}
	raw, err := json.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("activity repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, key, raw, 0); err != nil {
		return fmt.Errorf("activity repo: put: %w", err)
	}
	return nil
}

// pruneActivity evicts the oldest-seen members until len(seen) <= max.
func pruneActivity(seen map[int64]time.Time, max int) map[int64]time.Time {
	for len(seen) > max {
		var oldestID int64
		var oldestAt time.Time
		first := true
		for userID, at := range seen {
			if first || at.Before(oldestAt) {
				oldestID, oldestAt = userID, at
				first = false
			}
		}
		delete(seen, oldestID)
	}
	return seen
}
