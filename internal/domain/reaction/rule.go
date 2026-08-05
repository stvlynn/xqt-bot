// Package reaction holds keyword/regex-driven automatic emoji reactions.
package reaction

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind classifies a reaction rule.
type Kind string

const (
	KindKeyword Kind = "keyword" // substring match, earliest position wins
	KindRegex   Kind = "regex"   // RE2 match
)

// Rule maps a text trigger to an emoji reaction.
type Rule struct {
	Kind    Kind   `json:"kind"`
	Pattern string `json:"pattern"`
	Emoji   string `json:"emoji"`
}

// NewKeywordRule builds a keyword rule after validating the emoji.
func NewKeywordRule(keyword, emoji string) (Rule, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return Rule{}, fmt.Errorf("keyword must not be empty")
	}
	if !IsAllowedEmoji(emoji) {
		return Rule{}, fmt.Errorf("emoji %q is not usable as a Telegram reaction", emoji)
	}
	return Rule{Kind: KindKeyword, Pattern: keyword, Emoji: emoji}, nil
}

// NewRegexRule builds a regex rule after validating pattern and emoji.
func NewRegexRule(pattern, emoji string) (Rule, error) {
	if _, err := regexp.Compile(pattern); err != nil {
		return Rule{}, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	if !IsAllowedEmoji(emoji) {
		return Rule{}, fmt.Errorf("emoji %q is not usable as a Telegram reaction", emoji)
	}
	return Rule{Kind: KindRegex, Pattern: pattern, Emoji: emoji}, nil
}

// matchPosition returns the earliest hit position, or -1 for no match.
func (r Rule) matchPosition(text string) int {
	switch r.Kind {
	case KindKeyword:
		return strings.Index(text, r.Pattern)
	case KindRegex:
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return -1
		}
		loc := re.FindStringIndex(text)
		if loc == nil {
			return -1
		}
		return loc[0]
	}
	return -1
}

// Pick selects the matching rule whose trigger appears earliest in the text,
// mirroring tietie-bot's "first occurrence wins" semantics.
func Pick(rules []Rule, text string) (Rule, bool) {
	best := -1
	var bestRule Rule
	for _, r := range rules {
		pos := r.matchPosition(text)
		if pos >= 0 && (best < 0 || pos < best) {
			best = pos
			bestRule = r
		}
	}
	if best < 0 {
		return Rule{}, false
	}
	return bestRule, true
}
