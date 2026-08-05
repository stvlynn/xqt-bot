package application

import (
	"context"
	"errors"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// GroupMessagePipeline runs every inbound group text message through the
// per-message use cases in a fixed order: activity tracking, message logging,
// moderation, then auto-reaction (skipped when moderation hit). A failing
// step is recorded but never blocks the remaining steps.
type GroupMessagePipeline struct {
	mod   *ModerationService
	react *ReactionService
	sum   *SummaryService
	zom   *ZombieService
	now   func() time.Time
}

// NewGroupMessagePipeline builds the pipeline.
func NewGroupMessagePipeline(mod *ModerationService, react *ReactionService, sum *SummaryService, zom *ZombieService) *GroupMessagePipeline {
	return &GroupMessagePipeline{mod: mod, react: react, sum: sum, zom: zom, now: clockNow}
}

// HandleMessage processes one inbound group text message, aggregating the
// errors of any failing steps.
func (p *GroupMessagePipeline) HandleMessage(ctx context.Context, chatID, userID int64, messageID int, userName, text string) error {
	var errs []error

	if err := p.zom.Touch(ctx, chatID, userID); err != nil {
		errs = append(errs, err)
	}

	msg := summary.Message{
		MessageID: messageID,
		UserID:    userID,
		UserName:  userName,
		Text:      text,
		At:        p.now(),
	}
	if err := p.sum.RecordMessage(ctx, chatID, msg); err != nil {
		errs = append(errs, err)
	}

	hit := false
	if res, err := p.mod.CheckMessage(ctx, chatID, userID, messageID, text); err != nil {
		errs = append(errs, err)
	} else if res != nil && res.Hit {
		hit = true
	}

	if !hit {
		if err := p.react.OnMessage(ctx, chatID, messageID, text); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
