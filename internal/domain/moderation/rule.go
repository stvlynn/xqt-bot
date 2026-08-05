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

// BuiltinRules is the small built-in advertising/scam library. It targets
// patterns that are spam in virtually every group, independent of topic.
// Administrators extend it with their own words via /filter add.
func BuiltinRules() []FilterRule {
	words := []string{
		"刷单", "兼职加微", "加v信", "加vx", "日结工资",
		"usdt", "代充", "空投领取", "合约带单",
		"赌场", "博彩", "真人荷官",
	}
	regexps := []string{
		`(?i)\bt\.me/(joinchat|\+)[0-9a-z_-]+`,         // unsolicited group invite links
		`(?i)(whatsapp|telegram)\s*[+＋]?\d[\d\s-]{7,}`, // phone-number harvesting
	}
	rules := make([]FilterRule, 0, len(words)+len(regexps))
	for _, w := range words {
		r, err := NewWordRule(w)
		if err == nil {
			rules = append(rules, r)
		}
	}
	for _, p := range regexps {
		r, err := NewRegexRule(p)
		if err == nil {
			rules = append(rules, r)
		}
	}
	return rules
}
