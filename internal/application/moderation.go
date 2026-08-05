package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
)

// Filter-import tuning constants.
const (
	// maxFilterRules caps the total number of filter rules per chat.
	maxFilterRules = 5000
	// filterRefreshIntervalHours is the cadence of the per-chat word-list
	// refresh task created on the first import.
	filterRefreshIntervalHours = 24
)

// CheckResult describes what the moderation pipeline did with one message.
type CheckResult struct {
	Hit        bool
	Rule       moderation.FilterRule
	Deleted    bool
	MutedUntil time.Time // zero when the offender was not muted
}

// ImportResult summarizes one /filter import run.
type ImportResult struct {
	Added   int    // newly appended rules
	Skipped int    // fetched rules dropped as duplicates of existing ones
	Total   int    // total rule count after the import
	URL     string // the imported source
}

// RefreshResult summarizes one word-list refresh across all sources.
type RefreshResult struct {
	Sources int      // sources attempted
	Failed  []string // source URLs whose fetch/merge failed
	Added   int      // net change in total rule count (may be negative)
}

// ModerationService runs the sensitive-word/regex filter and the admin
// moderation commands (kick/ban/mute).
type ModerationService struct {
	settings ports.SettingsRepository
	tasks    ports.TaskRepository
	tg       ports.TelegramGateway
	wordlist ports.WordListGateway
	now      func() time.Time
}

// NewModerationService builds the service.
func NewModerationService(settings ports.SettingsRepository, tasks ports.TaskRepository, tg ports.TelegramGateway, wordlist ports.WordListGateway) *ModerationService {
	return &ModerationService{settings: settings, tasks: tasks, tg: tg, wordlist: wordlist, now: clockNow}
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

// ImportWordList fetches a remote word list and merges it into the chat's
// filter rules: rules previously imported from the same URL are replaced,
// fetched rules conflicting (kind+pattern) with existing ones are skipped.
// The first import of a chat also schedules the daily filter_refresh task.
func (s *ModerationService) ImportWordList(ctx context.Context, chatID, requesterID int64, url string) (*ImportResult, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, ErrInvalidArgument
	}
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return nil, err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	firstSource := len(st.Filter.Sources) == 0
	added, skipped, err := s.mergeSource(ctx, st, url)
	if err != nil {
		return nil, err
	}
	known := false
	for _, src := range st.Filter.Sources {
		if src == url {
			known = true
			break
		}
	}
	if !known {
		st.Filter.Sources = append(st.Filter.Sources, url)
	}
	if err := s.settings.Save(ctx, st); err != nil {
		return nil, err
	}
	if firstSource {
		task := schedule.NewTask(schedule.KindFilterRefresh, chatID, filterRefreshIntervalHours, s.now())
		if err := s.tasks.Save(ctx, task); err != nil {
			return nil, err
		}
	}
	return &ImportResult{Added: added, Skipped: skipped, Total: len(st.Filter.Rules), URL: url}, nil
}

// RefreshWordLists re-fetches every imported source (admin command variant).
func (s *ModerationService) RefreshWordLists(ctx context.Context, chatID, requesterID int64) (*RefreshResult, error) {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return nil, err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	if len(st.Filter.Sources) == 0 {
		return nil, ErrNotFound
	}
	return s.refreshSources(ctx, st)
}

// RefreshAllWordLists is the system-task variant of RefreshWordLists: no
// admin check, and a chat without sources is a no-op rather than an error.
func (s *ModerationService) RefreshAllWordLists(ctx context.Context, chatID int64) (*RefreshResult, error) {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	if len(st.Filter.Sources) == 0 {
		return &RefreshResult{}, nil
	}
	return s.refreshSources(ctx, st)
}

// refreshSources merges every source in the settings; one failing source is
// recorded and never aborts the others.
func (s *ModerationService) refreshSources(ctx context.Context, st *chat.Settings) (*RefreshResult, error) {
	res := &RefreshResult{Sources: len(st.Filter.Sources)}
	before := len(st.Filter.Rules)
	for _, url := range st.Filter.Sources {
		if _, _, err := s.mergeSource(ctx, st, url); err != nil {
			res.Failed = append(res.Failed, url)
		}
	}
	res.Added = len(st.Filter.Rules) - before
	if err := s.settings.Save(ctx, st); err != nil {
		return nil, err
	}
	return res, nil
}

// mergeSource replaces all rules previously imported from url with a freshly
// fetched set. The settings aggregate is mutated only on success.
func (s *ModerationService) mergeSource(ctx context.Context, st *chat.Settings, url string) (added, skipped int, err error) {
	fetched, err := s.wordlist.Fetch(ctx, url)
	if err != nil {
		return 0, 0, err
	}
	type ruleKey struct {
		kind    moderation.RuleKind
		pattern string
	}
	kept := make([]moderation.FilterRule, 0, len(st.Filter.Rules)+len(fetched))
	seen := make(map[ruleKey]struct{}, len(st.Filter.Rules)+len(fetched))
	for _, r := range st.Filter.Rules {
		if r.Source == url {
			continue // stale import from this source, replaced below
		}
		kept = append(kept, r)
		seen[ruleKey{r.Kind, r.Pattern}] = struct{}{}
	}
	for _, r := range fetched {
		k := ruleKey{r.Kind, r.Pattern}
		if _, dup := seen[k]; dup {
			skipped++
			continue
		}
		seen[k] = struct{}{}
		kept = append(kept, r)
		added++
	}
	if len(kept) > maxFilterRules {
		return 0, 0, fmt.Errorf("moderation: filter rule limit of %d exceeded", maxFilterRules)
	}
	st.Filter.Rules = kept
	return added, skipped, nil
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
