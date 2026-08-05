package bot

import "testing"

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{"not a command", "hello", "", "", false},
		{"empty", "", "", "", false},
		{"bare slash", "/", "", "", false},
		{"simple", "/start", "start", "", true},
		{"with args", "/mute 10", "mute", "10", true},
		{"extra spaces", "/pick  a  b  c ", "pick", "a  b  c", true},
		{"bot suffix", "/kick@xqtttbot", "kick", "", true},
		{"bot suffix with args", "/filter@xqtttbot add 刷单", "filter", "add 刷单", true},
		{"upper case", "/SUMMARY 12", "summary", "12", true},
		{"regex args", "/filter add /a b/", "filter", "add /a b/", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args, ok := parseCommand(tt.text)
			if name != tt.wantName || args != tt.wantArgs || ok != tt.wantOK {
				t.Fatalf("parseCommand(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.text, name, args, ok, tt.wantName, tt.wantArgs, tt.wantOK)
			}
		})
	}
}

func TestCommandTarget(t *testing.T) {
	tests := []struct {
		text string
		want string
	}{
		{"/kick@XqtTtBot", "xqtttbot"},
		{"/kick", ""},
		{"/kick@xqtttbot 10", "xqtttbot"},
		{"not a command", ""},
		{"/pick a@b c", ""}, // @ only meaningful in the first token
	}
	for _, tt := range tests {
		if got := commandTarget(tt.text); got != tt.want {
			t.Errorf("commandTarget(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestCaptchaDataRoundTrip(t *testing.T) {
	data := encodeCaptchaData(-100123, 3)
	userID, idx, ok := parseCaptchaData(data)
	if !ok || userID != -100123 || idx != 3 {
		t.Fatalf("round trip failed: got (%d, %d, %v)", userID, idx, ok)
	}
}

func TestParseCaptchaDataInvalid(t *testing.T) {
	for _, data := range []string{
		"", "c:", "c:abc:1", "c:1:", "c:1:x", "c:1:-1", "m:cap", "c:1:2:3", "c:1",
	} {
		if _, _, ok := parseCaptchaData(data); ok {
			t.Errorf("parseCaptchaData(%q) unexpectedly ok", data)
		}
	}
}

func TestParsePanelData(t *testing.T) {
	action, ok := parsePanelData("m:cap")
	if !ok || action != "cap" {
		t.Fatalf("parsePanelData(m:cap) = (%q, %v)", action, ok)
	}
	for _, data := range []string{"", "m:", "c:1:2", "x:cap"} {
		if _, ok := parsePanelData(data); ok {
			t.Errorf("parsePanelData(%q) unexpectedly ok", data)
		}
	}
}

func TestSplitPatternArg(t *testing.T) {
	tests := []struct {
		arg         string
		wantPattern string
		wantRegex   bool
		wantOK      bool
	}{
		{"刷单", "刷单", false, true},
		{"/a+b/", "a+b", true, true},
		{"/(?i)spam \\d+/", "(?i)spam \\d+", true, true},
		{"", "", false, false},
		{"  ", "", false, false},
		{"/", "/", false, true}, // a lone slash is a plain keyword
		{"//", "", false, false},
		{" /re/ ", "re", true, true},
	}
	for _, tt := range tests {
		pattern, isRegex, ok := splitPatternArg(tt.arg)
		if pattern != tt.wantPattern || isRegex != tt.wantRegex || ok != tt.wantOK {
			t.Errorf("splitPatternArg(%q) = (%q, %v, %v), want (%q, %v, %v)",
				tt.arg, pattern, isRegex, ok, tt.wantPattern, tt.wantRegex, tt.wantOK)
		}
	}
}

func TestParseAutoReactArgs(t *testing.T) {
	tests := []struct {
		args        string
		wantPattern string
		wantEmoji   string
		wantRegex   bool
		wantOK      bool
	}{
		{"早上好 ☀️", "早上好", "☀️", false, true},
		{"/a+b/ 👍", "a+b", "👍", true, true},
		{"/spam \\d+/ 🎉", "spam \\d+", "🎉", true, true},
		{"只有一个词", "", "", false, false},
		{"", "", "", false, false},
		{" 👍", "", "", false, false},
	}
	for _, tt := range tests {
		pattern, emoji, isRegex, ok := parseAutoReactArgs(tt.args)
		if pattern != tt.wantPattern || emoji != tt.wantEmoji || isRegex != tt.wantRegex || ok != tt.wantOK {
			t.Errorf("parseAutoReactArgs(%q) = (%q, %q, %v, %v), want (%q, %q, %v, %v)",
				tt.args, pattern, emoji, isRegex, ok, tt.wantPattern, tt.wantEmoji, tt.wantRegex, tt.wantOK)
		}
	}
}
