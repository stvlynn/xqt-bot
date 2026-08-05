package moderation

import "strings"

// ParseWordList parses a remote word-list document into filter rules tagged
// with the given source (usually the list URL; empty for manual input).
//
// Format: one rule per line. Blank lines and lines starting with '#' are
// ignored. A line wrapped in slashes ("/pattern/") is a RE2 regular
// expression; anything else is a case-insensitive word. Lines whose regex
// does not compile are skipped, as are duplicate (kind, pattern) pairs; both
// are counted in the skipped return value.
func ParseWordList(body, source string) (rules []FilterRule, skipped int) {
	type key struct {
		kind    RuleKind
		pattern string
	}
	seen := make(map[key]struct{})
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var (
			r   FilterRule
			err error
		)
		if len(line) >= 2 && strings.HasPrefix(line, "/") && strings.HasSuffix(line, "/") {
			inner := line[1 : len(line)-1]
			if inner == "" {
				skipped++
				continue
			}
			r, err = NewRegexRule(inner)
		} else {
			r, err = NewWordRule(line)
		}
		if err != nil {
			skipped++
			continue
		}
		r.Source = source
		k := key{r.Kind, r.Pattern}
		if _, dup := seen[k]; dup {
			skipped++
			continue
		}
		seen[k] = struct{}{}
		rules = append(rules, r)
	}
	return rules, skipped
}
