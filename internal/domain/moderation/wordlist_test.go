package moderation

import "testing"

func TestParseWordListMixedContent(t *testing.T) {
	body := `# comment line
刷单
/(?i)\bt\.me/(joinchat|\+)[0-9a-z_-]+/

usdt
`
	rules, skipped := ParseWordList(body, "https://example.com/list.txt")
	if skipped != 0 {
		t.Fatalf("want 0 skipped, got %d", skipped)
	}
	if len(rules) != 3 {
		t.Fatalf("want 3 rules, got %+v", rules)
	}
	if rules[0].Kind != RuleWord || rules[0].Pattern != "刷单" {
		t.Fatalf("unexpected word rule: %+v", rules[0])
	}
	if rules[1].Kind != RuleRegex || rules[1].Pattern != `(?i)\bt\.me/(joinchat|\+)[0-9a-z_-]+` {
		t.Fatalf("unexpected regex rule: %+v", rules[1])
	}
	for _, r := range rules {
		if r.Source != "https://example.com/list.txt" {
			t.Fatalf("rule missing source: %+v", r)
		}
	}
}

func TestParseWordListSkipsInvalidRegex(t *testing.T) {
	rules, skipped := ParseWordList("/unclosed(/\n//\nok", "")
	if len(rules) != 1 || rules[0].Pattern != "ok" || rules[0].Kind != RuleWord {
		t.Fatalf("unexpected rules: %+v", rules)
	}
	if skipped != 2 { // bad regex + empty "//"
		t.Fatalf("want 2 skipped, got %d", skipped)
	}
}

func TestParseWordListDeduplicates(t *testing.T) {
	// Same pattern as word and regex is NOT a duplicate (different kind).
	rules, skipped := ParseWordList("spam\nspam\n/spam/\n/spam/", "src")
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %+v", rules)
	}
	if skipped != 2 {
		t.Fatalf("want 2 skipped duplicates, got %d", skipped)
	}
}

func TestParseWordListTrimsWhitespace(t *testing.T) {
	rules, skipped := ParseWordList("  刷单  \n\t/usdt/\t", "")
	if skipped != 0 || len(rules) != 2 {
		t.Fatalf("got rules=%+v skipped=%d", rules, skipped)
	}
	if rules[0].Pattern != "刷单" || rules[1].Pattern != "usdt" || rules[1].Kind != RuleRegex {
		t.Fatalf("unexpected rules: %+v", rules)
	}
}

func TestParseWordListEmptyBody(t *testing.T) {
	rules, skipped := ParseWordList("", "src")
	if len(rules) != 0 || skipped != 0 {
		t.Fatalf("got rules=%+v skipped=%d", rules, skipped)
	}
}

func TestParseWordListSingleSlashIsWord(t *testing.T) {
	// A lone "/" is not slash-wrapped (needs >= 2 chars), so it is a word.
	rules, skipped := ParseWordList("/", "")
	if skipped != 0 || len(rules) != 1 || rules[0].Kind != RuleWord || rules[0].Pattern != "/" {
		t.Fatalf("got rules=%+v skipped=%d", rules, skipped)
	}
}

func TestParseWordListParsedRulesMatch(t *testing.T) {
	rules, _ := ParseWordList("刷单\n/(?i)usdt/", "src")
	if _, ok := MatchAny(rules, "兼职刷单日结"); !ok {
		t.Fatalf("word rule should match")
	}
	if _, ok := MatchAny(rules, "buy USDT now"); !ok {
		t.Fatalf("regex rule should match")
	}
	if _, ok := MatchAny(rules, "clean text"); ok {
		t.Fatalf("clean text must not match")
	}
}
