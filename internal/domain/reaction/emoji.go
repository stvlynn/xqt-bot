package reaction

// allowedEmojis is the subset of Telegram's reaction whitelist that we accept
// in rules. Telegram rejects anything outside its own whitelist; keeping the
// common set here gives users clear feedback at rule-creation time.
var allowedEmojis = map[string]bool{}

// AllowedEmojis returns the usable reaction emojis (also fed to the LLM as
// the only choices it may pick from).
func AllowedEmojis() []string {
	list := []string{
		"👍", "👎", "❤️", "🔥", "🥰", "👏", "😁", "🤔", "🤯", "😱",
		"🤬", "😢", "🎉", "🤩", "🤮", "💩", "🙏", "👌", "🕊️", "🤡",
		"🥱", "🥴", "😍", "🐳", "❤️‍🔥", "🌚", "🌭", "💯", "🤣", "⚡",
		"🍌", "🏆", "💔", "🤨", "😐", "🍓", "🍾", "💋", "🖕", "😈",
		"😴", "😭", "🤓", "👻", "👨‍💻", "👀", "🎃", "🙈", "😇", "😨",
		"🤝", "✍️", "🤗", "🫡", "🎅", "🎄", "☃️", "💅", "🤪", "🗿",
		"🆒", "💘", "🙉", "🦄", "😘", "💊", "🙊", "😎", "👾", "🤷‍♂️",
		"🤷", "🤷‍♀️", "😡",
	}
	return list
}

func init() {
	for _, e := range AllowedEmojis() {
		allowedEmojis[e] = true
	}
}

// IsAllowedEmoji reports whether e may be used in a reaction rule.
func IsAllowedEmoji(e string) bool {
	return allowedEmojis[e]
}
