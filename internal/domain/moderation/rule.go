// Package moderation holds the spam/advertising filter rules and the
// join-captcha challenge model.
package moderation

import (
	"fmt"
	"regexp"
	"strings"
)

// RuleKind classifies a filter rule.
type RuleKind string

const (
	RuleWord  RuleKind = "word"  // case-insensitive substring match
	RuleRegex RuleKind = "regex" // RE2 regular expression
)

// FilterRule matches one message text against a word or a pattern.
type FilterRule struct {
	Kind    RuleKind `json:"kind"`
	Pattern string   `json:"pattern"`
	// Source is empty for rules added by hand (/filter add) and holds the
	// word-list URL for rules imported via /filter import.
	Source string `json:"source,omitempty"`
}

// NewWordRule builds a substring rule. It rejects empty words.
func NewWordRule(word string) (FilterRule, error) {
	word = strings.TrimSpace(word)
	if word == "" {
		return FilterRule{}, fmt.Errorf("filter word must not be empty")
	}
	return FilterRule{Kind: RuleWord, Pattern: word}, nil
}

// NewRegexRule builds a regex rule, validating the pattern eagerly.
func NewRegexRule(pattern string) (FilterRule, error) {
	if _, err := regexp.Compile(pattern); err != nil {
		return FilterRule{}, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	return FilterRule{Kind: RuleRegex, Pattern: pattern}, nil
}

// Match reports whether the rule hits the given text.
func (r FilterRule) Match(text string) bool {
	switch r.Kind {
	case RuleWord:
		return strings.Contains(strings.ToLower(text), strings.ToLower(r.Pattern))
	case RuleRegex:
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return false // persisted rules are validated on write; be safe anyway
		}
		return re.MatchString(text)
	}
	return false
}

// MatchAny returns the first rule that hits the text, if any.
func MatchAny(rules []FilterRule, text string) (FilterRule, bool) {
	for _, r := range rules {
		if r.Match(text) {
			return r, true
		}
	}
	return FilterRule{}, false
}
