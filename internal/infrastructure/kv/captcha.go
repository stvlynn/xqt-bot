package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// listExpiredCap bounds one expiry sweep (per ports.CaptchaRepository).
const listExpiredCap = 100

// captchaTTLGrace extends the storage TTL past the session expiry so the
// sweeper can still find and punish expired sessions.
const captchaTTLGrace = 60

// CaptchaRepository is a Store-backed ports.CaptchaRepository.
type CaptchaRepository struct {
	store Store
}

// NewCaptchaRepository creates the repository over the given Store.
func NewCaptchaRepository(store Store) *CaptchaRepository {
	return &CaptchaRepository{store: store}
}

// CaptchaPrefix selects all captcha session keys.
const CaptchaPrefix = "captcha:"

// CaptchaKey builds the storage key for one member's pending session.
func CaptchaKey(chatID, userID int64) string {
	return fmt.Sprintf("%s%d:%d", CaptchaPrefix, chatID, userID)
}

// Save persists a pending session. The storage TTL mirrors the challenge
// timeout (ExpiresAt) plus a grace window, so entries clean themselves up
// even if the sweeper never runs.
func (r *CaptchaRepository) Save(ctx context.Context, s *moderation.Session) error {
	raw, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("captcha repo: encode: %w", err)
	}
	ttl := int(time.Until(s.ExpiresAt).Seconds()) + captchaTTLGrace
	if ttl < captchaTTLGrace {
		ttl = captchaTTLGrace
	}
	if err := r.store.Put(ctx, CaptchaKey(s.ChatID, s.UserID), raw, ttl); err != nil {
		return fmt.Errorf("captcha repo: put: %w", err)
	}
	return nil
}

// Get loads one pending session.
func (r *CaptchaRepository) Get(ctx context.Context, chatID, userID int64) (*moderation.Session, error) {
	raw, err := r.store.Get(ctx, CaptchaKey(chatID, userID))
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return nil, ports.ErrNotFound
		}
		return nil, fmt.Errorf("captcha repo: get: %w", err)
	}
	var s moderation.Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("captcha repo: decode %d/%d: %w", chatID, userID, err)
	}
	return &s, nil
}

// Delete removes a resolved session.
func (r *CaptchaRepository) Delete(ctx context.Context, chatID, userID int64) error {
	if err := r.store.Delete(ctx, CaptchaKey(chatID, userID)); err != nil {
		return fmt.Errorf("captcha repo: delete: %w", err)
	}
	return nil
}

// ListExpired returns up to listExpiredCap sessions that expired before now.
func (r *CaptchaRepository) ListExpired(ctx context.Context, now time.Time) ([]moderation.Session, error) {
	keys, err := r.store.ListKeys(ctx, CaptchaPrefix)
	if err != nil {
		return nil, fmt.Errorf("captcha repo: list: %w", err)
	}
	expired := make([]moderation.Session, 0)
	for _, key := range keys {
		raw, err := r.store.Get(ctx, key)
		if err != nil {
			continue // vanished or transient error; next sweep will retry
		}
		var s moderation.Session
		if err := json.Unmarshal(raw, &s); err != nil {
			continue // corrupt entry; do not block the sweep
		}
		if s.Expired(now) {
			expired = append(expired, s)
			if len(expired) >= listExpiredCap {
				break
			}
		}
	}
	return expired, nil
}
