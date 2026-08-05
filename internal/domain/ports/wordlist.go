package ports

import (
	"context"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
)

// WordListGateway downloads a remote sensitive-word list and returns the
// parsed rules, each tagged with the list URL in FilterRule.Source.
type WordListGateway interface {
	Fetch(ctx context.Context, url string) ([]moderation.FilterRule, error)
}
