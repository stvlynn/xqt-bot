package application

import (
	"context"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// CheckResult describes what the moderation pipeline did with one message.
type CheckResult struct {
	Hit        bool
	Rule       moderation.FilterRule
	Deleted    bool
	MutedUntil time.Time // zero when the offender was not muted
}

// ModerationService runs the sensitive-word/regex filter and the admin
// moderation commands (kick/ban/mute).
type ModerationService struct {
	settings ports.SettingsRepository
	tg       ports.TelegramGateway
	now      func() time.Time
}

// NewModerationService builds the service.
func NewModerationService(settings ports.SettingsRepository, tg ports.TelegramGateway) *ModerationService {
	return &ModerationService{settings: settings, tg: tg, now: clockNow}
}

// CheckMessage applies the chat's filter rules to one incoming message.
// On a hit it deletes and/or mutes according to the filter configuration.
func (s *ModerationService) CheckMessage(ctx context.Context, chatID, userID int64, messageID int, text string) (*CheckResult, error) {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	if !st.Filter.Enabled {
		return &CheckResult{Hit: false}, nil
	}
	rule, ok := moderation.MatchAny(st.Filter.Rules, text)
	if !ok {
		return &CheckResult{Hit: false}, nil
	}
	res := &CheckResult{Hit: true, Rule: rule}
	if st.Filter.DeleteMessage {
		if err := s.tg.DeleteMessage(ctx, chatID, messageID); err != nil {
			return nil, err
		}
		res.Deleted = true
	}
	if st.Filter.MuteMinutes > 0 {
		until := s.now().Add(time.Duration(st.Filter.MuteMinutes) * time.Minute)
		if err := s.tg.RestrictMember(ctx, chatID, userID, false, until); err != nil {
			return nil, err
		}
		res.MutedUntil = until
	}
	return res, nil
}

// AddWordRule appends a case-insensitive substring rule.
func (s *ModerationService) AddWordRule(ctx context.Context, chatID, requesterID int64, pattern string) error {
	rule, err := moderation.NewWordRule(pattern)
	if err != nil {
		return err
	}
	return s.addRule(ctx, chatID, requesterID, rule)
}

// AddRegexRule appends a validated RE2 rule.
func (s *ModerationService) AddRegexRule(ctx context.Context, chatID, requesterID int64, pattern string) error {
	rule, err := moderation.NewRegexRule(pattern)
	if err != nil {
		return err
	}
	return s.addRule(ctx, chatID, requesterID, rule)
}

func (s *ModerationService) addRule(ctx context.Context, chatID, requesterID int64, rule moderation.FilterRule) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	for _, r := range st.Filter.Rules {
		if r.Kind == rule.Kind && r.Pattern == rule.Pattern {
			return ErrDuplicate
		}
	}
	st.Filter.Rules = append(st.Filter.Rules, rule)
	return s.settings.Save(ctx, st)
}

// RemoveRule deletes the first rule with the given pattern.
func (s *ModerationService) RemoveRule(ctx context.Context, chatID, requesterID int64, pattern string) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range st.Filter.Rules {
		if r.Pattern == pattern {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	rules := make([]moderation.FilterRule, 0, len(st.Filter.Rules)-1)
	rules = append(rules, st.Filter.Rules[:idx]...)
	rules = append(rules, st.Filter.Rules[idx+1:]...)
	st.Filter.Rules = rules
	return s.settings.Save(ctx, st)
}

// SetFilterEnabled toggles the whole filter pipeline for the chat.
func (s *ModerationService) SetFilterEnabled(ctx context.Context, chatID, requesterID int64, enabled bool) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	st.Filter.Enabled = enabled
	return s.settings.Save(ctx, st)
}

// SetMuteMinutes sets how long filter offenders are muted (0 = delete only).
func (s *ModerationService) SetMuteMinutes(ctx context.Context, chatID, requesterID int64, minutes int) error {
	if minutes < 0 {
		return ErrInvalidArgument
	}
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	st.Filter.MuteMinutes = minutes
	return s.settings.Save(ctx, st)
}

// Kick removes the target from the chat without banning them (ban + unban).
func (s *ModerationService) Kick(ctx context.Context, chatID, requesterID, targetUserID int64) error {
	if err := s.guardTarget(ctx, chatID, requesterID, targetUserID); err != nil {
		return err
	}
	if err := s.tg.BanMember(ctx, chatID, targetUserID, false); err != nil {
		return err
	}
	return s.tg.UnbanMember(ctx, chatID, targetUserID)
}

// Ban bans the target and deletes their message history.
func (s *ModerationService) Ban(ctx context.Context, chatID, requesterID, targetUserID int64) error {
	if err := s.guardTarget(ctx, chatID, requesterID, targetUserID); err != nil {
		return err
	}
	return s.tg.BanMember(ctx, chatID, targetUserID, true)
}

// Mute mutes the target for the given number of minutes.
func (s *ModerationService) Mute(ctx context.Context, chatID, requesterID, targetUserID int64, minutes int) error {
	if minutes <= 0 {
		return ErrInvalidArgument
	}
	if err := s.guardTarget(ctx, chatID, requesterID, targetUserID); err != nil {
		return err
	}
	return s.tg.RestrictMember(ctx, chatID, targetUserID, false, s.now().Add(time.Duration(minutes)*time.Minute))
}

// Unmute lifts a mute on the target.
func (s *ModerationService) Unmute(ctx context.Context, chatID, requesterID, targetUserID int64) error {
	if err := s.guardTarget(ctx, chatID, requesterID, targetUserID); err != nil {
		return err
	}
	return s.tg.RestrictMember(ctx, chatID, targetUserID, true, s.now())
}

// guardTarget enforces that the requester is an admin and the target is not.
func (s *ModerationService) guardTarget(ctx context.Context, chatID, requesterID, targetUserID int64) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	ok, err := s.tg.IsAdmin(ctx, chatID, targetUserID)
	if err != nil {
		return err
	}
	if ok {
		return ErrTargetIsAdmin
	}
	return nil
}
