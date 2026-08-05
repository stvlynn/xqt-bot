package application

import (
	"context"
	"sync"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/reaction"
)

// llmReactionCooldown is the minimum gap between two LLM-picked reactions in
// one chat, keeping costs and noise bounded. Tracked in process memory: the
// WASM isolate is long-lived enough for this to be effective.
const llmReactionCooldown = 30 * time.Second

// ReactionService attaches emoji reactions to messages via keyword/regex
// rules, optionally falling back to an LLM pick when no rule matches.
type ReactionService struct {
	settings ports.SettingsRepository
	tg       ports.TelegramGateway
	llm      ports.LLMGateway
	now      func() time.Time

	mu           sync.Mutex
	lastLLMReact map[int64]time.Time
}

// NewReactionService builds the service. llm may be nil when no backend is
// configured; LLM fallback is then silently skipped.
func NewReactionService(settings ports.SettingsRepository, tg ports.TelegramGateway, llm ports.LLMGateway) *ReactionService {
	return &ReactionService{
		settings:     settings,
		tg:           tg,
		llm:          llm,
		now:          clockNow,
		lastLLMReact: make(map[int64]time.Time),
	}
}

// OnMessage reacts to one group message when a rule hits or the LLM fallback
// decides to. Failures to set a reaction are reported but never fatal.
func (s *ReactionService) OnMessage(ctx context.Context, chatID int64, messageID int, text string) error {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	if rule, ok := reaction.Pick(st.AutoReact.Rules, text); ok {
		return s.tg.SetReaction(ctx, chatID, messageID, rule.Emoji)
	}
	if !st.AutoReact.LLMEnabled || s.llm == nil || !s.llm.Available() || !s.cooledDown(chatID) {
		return nil
	}
	emoji, ok, err := s.llm.PickReaction(ctx, text, reaction.AllowedEmojis())
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if err := s.tg.SetReaction(ctx, chatID, messageID, emoji); err != nil {
		return err
	}
	s.markReacted(chatID)
	return nil
}

// cooledDown reports whether the chat's LLM cooldown has elapsed.
func (s *ReactionService) cooledDown(chatID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	last, ok := s.lastLLMReact[chatID]
	return !ok || s.now().Sub(last) >= llmReactionCooldown
}

// markReacted records a successful LLM reaction for cooldown purposes.
func (s *ReactionService) markReacted(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastLLMReact[chatID] = s.now()
}

// AddKeywordRule appends a keyword reaction rule.
func (s *ReactionService) AddKeywordRule(ctx context.Context, chatID, requesterID int64, keyword, emoji string) error {
	rule, err := reaction.NewKeywordRule(keyword, emoji)
	if err != nil {
		return err
	}
	return s.addRule(ctx, chatID, requesterID, rule)
}

// AddRegexRule appends a regex reaction rule.
func (s *ReactionService) AddRegexRule(ctx context.Context, chatID, requesterID int64, pattern, emoji string) error {
	rule, err := reaction.NewRegexRule(pattern, emoji)
	if err != nil {
		return err
	}
	return s.addRule(ctx, chatID, requesterID, rule)
}

func (s *ReactionService) addRule(ctx context.Context, chatID, requesterID int64, rule reaction.Rule) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	for _, r := range st.AutoReact.Rules {
		if r.Kind == rule.Kind && r.Pattern == rule.Pattern {
			return ErrDuplicate
		}
	}
	st.AutoReact.Rules = append(st.AutoReact.Rules, rule)
	return s.settings.Save(ctx, st)
}

// RemoveRule deletes the first reaction rule with the given pattern.
func (s *ReactionService) RemoveRule(ctx context.Context, chatID, requesterID int64, pattern string) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	idx := -1
	for i, r := range st.AutoReact.Rules {
		if r.Pattern == pattern {
			idx = i
			break
		}
	}
	if idx < 0 {
		return ErrNotFound
	}
	rules := make([]reaction.Rule, 0, len(st.AutoReact.Rules)-1)
	rules = append(rules, st.AutoReact.Rules[:idx]...)
	rules = append(rules, st.AutoReact.Rules[idx+1:]...)
	st.AutoReact.Rules = rules
	return s.settings.Save(ctx, st)
}

// SetLLMEnabled toggles the LLM reaction fallback for the chat.
func (s *ReactionService) SetLLMEnabled(ctx context.Context, chatID, requesterID int64, enabled bool) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	st.AutoReact.LLMEnabled = enabled
	return s.settings.Save(ctx, st)
}
