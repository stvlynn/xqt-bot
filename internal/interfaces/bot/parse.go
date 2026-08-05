package bot

import (
	"fmt"
	"strconv"
	"strings"
)

// Callback-data prefixes. Captcha answers carry the target user and option
// index; panel actions carry the toggle name.
const (
	captchaCallbackPrefix = "c:"
	panelCallbackPrefix   = "m:"
)

// parseCommand splits a command message into the lower-cased command name
// (without the leading "/" and without any "@botname" suffix) and the raw,
// trimmed argument string. ok is false when the text is not a command.
func parseCommand(text string) (name, args string, ok bool) {
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	body := text[1:]
	name, args, _ = strings.Cut(body, " ")
	args = strings.TrimSpace(args)
	if i := strings.IndexByte(name, '@'); i >= 0 {
		name = name[:i]
	}
	name = strings.ToLower(name)
	if name == "" {
		return "", "", false
	}
	return name, args, true
}

// commandTarget returns the lower-cased "@botname" suffix of a command's
// first token, or "" when the command names no bot. It lets the handler
// ignore commands addressed at other bots in the same group.
func commandTarget(text string) string {
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	name, _, _ := strings.Cut(text[1:], " ")
	i := strings.IndexByte(name, '@')
	if i < 0 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

// encodeCaptchaData builds the callback payload for one answer option.
func encodeCaptchaData(userID int64, optionIndex int) string {
	return fmt.Sprintf("%s%d:%d", captchaCallbackPrefix, userID, optionIndex)
}

// parseCaptchaData decodes a "c:<userID>:<optionIndex>" callback payload.
func parseCaptchaData(data string) (userID int64, optionIndex int, ok bool) {
	rest, found := strings.CutPrefix(data, captchaCallbackPrefix)
	if !found {
		return 0, 0, false
	}
	uid, idx, found := strings.Cut(rest, ":")
	if !found {
		return 0, 0, false
	}
	userID, err := strconv.ParseInt(uid, 10, 64)
	if err != nil {
		return 0, 0, false
	}
	optionIndex, err = strconv.Atoi(idx)
	if err != nil || optionIndex < 0 {
		return 0, 0, false
	}
	return userID, optionIndex, true
}

// parsePanelData decodes a "m:<action>" admin-panel callback payload.
func parsePanelData(data string) (action string, ok bool) {
	action, ok = strings.CutPrefix(data, panelCallbackPrefix)
	return action, ok && action != ""
}

// splitPatternArg interprets one rule argument: wrapped in slashes it is a
// regex (the slashes are stripped), otherwise a plain keyword.
func splitPatternArg(arg string) (pattern string, isRegex bool, ok bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", false, false
	}
	if len(arg) >= 2 && strings.HasPrefix(arg, "/") && strings.HasSuffix(arg, "/") {
		inner := arg[1 : len(arg)-1]
		if inner == "" {
			return "", false, false
		}
		return inner, true, true
	}
	return arg, false, true
}

// parseAutoReactArgs splits "<pattern> <emoji>" for /autoreact. The emoji is
// the last whitespace-separated token; everything before it is the pattern
// (which may be a slash-wrapped regex containing spaces).
func parseAutoReactArgs(args string) (pattern, emoji string, isRegex, ok bool) {
	args = strings.TrimSpace(args)
	idx := strings.LastIndexAny(args, " \t")
	if idx <= 0 {
		return "", "", false, false
	}
	emoji = strings.TrimSpace(args[idx+1:])
	pattern, isRegex, ok = splitPatternArg(args[:idx])
	return pattern, emoji, isRegex, ok
}
