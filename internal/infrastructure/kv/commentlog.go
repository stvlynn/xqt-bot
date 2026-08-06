package kv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// CommentLogTTLSeconds is how long comment previews are kept per post.
const CommentLogTTLSeconds = 7 * 24 * 3600

// CommentLogRepository is a Store-backed ports.CommentLogRepository.
type CommentLogRepository struct {
	store Store
}

// NewCommentLogRepository creates the repository over the given Store.
func NewCommentLogRepository(store Store) *CommentLogRepository {
	return &CommentLogRepository{store: store}
}

// CommentLogPrefix selects all comment-log keys.
const CommentLogPrefix = "comments:"

// CommentLogKey builds the storage key for one post's comment previews.
func CommentLogKey(channelID int64, postID int) string {
	return fmt.Sprintf("%s%d:%d", CommentLogPrefix, channelID, postID)
}

// Append records one comment preview, letting the domain log trim the text
// and evict the oldest entries beyond its capacity.
func (r *CommentLogRepository) Append(ctx context.Context, channelID int64, postID int, c channelpost.Comment) error {
	log, err := r.load(ctx, channelID, postID)
	if err != nil {
		return err
	}
	log.Add(c)
	raw, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("comments repo: encode: %w", err)
	}
	if err := r.store.Put(ctx, CommentLogKey(channelID, postID), raw, CommentLogTTLSeconds); err != nil {
		return fmt.Errorf("comments repo: put: %w", err)
	}
	return nil
}

// List implements ports.CommentLogRepository.
func (r *CommentLogRepository) List(ctx context.Context, channelID int64, postID int) ([]channelpost.Comment, error) {
	log, err := r.load(ctx, channelID, postID)
	if err != nil {
		return nil, err
	}
	return log.Comments, nil
}

// load fetches the log, transparently creating an empty one on a miss.
func (r *CommentLogRepository) load(ctx context.Context, channelID int64, postID int) (*channelpost.CommentLog, error) {
	raw, err := r.store.Get(ctx, CommentLogKey(channelID, postID))
	switch {
	case err == nil:
		var log channelpost.CommentLog
		if err := json.Unmarshal(raw, &log); err != nil {
			return nil, fmt.Errorf("comments repo: decode %d/%d: %w", channelID, postID, err)
		}
		return &log, nil
	case errors.Is(err, ports.ErrNotFound):
		return &channelpost.CommentLog{}, nil
	default:
		return nil, fmt.Errorf("comments repo: get %d/%d: %w", channelID, postID, err)
	}
}
