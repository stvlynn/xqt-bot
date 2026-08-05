// Package bot adapts Telegram updates to the application layer: it parses
// commands and callbacks, calls use-case services, and renders replies from
// the templates in texts.go. No Telegram Bot API calls happen here directly —
// everything goes through ports.TelegramGateway.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stvlynn/xqt-bot/internal/application"
	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/reaction"
)

// Admin-panel callback actions (the <action> in "m:<action>").
const (
	panelActionCaptcha = "cap"
	panelActionFilter  = "filter"
	panelActionLLM     = "llm"
	panelActionSummary = "sum"
	panelActionRefresh = "refresh"
)

// defaultMuteMinutes is the /mute duration when the admin gives none.
const defaultMuteMinutes = 10

// Deps wires everything the Handler needs; all services are built in main.
type Deps struct {
	Telegram    ports.TelegramGateway
	LLM         ports.LLMGateway // may be nil when unconfigured
	Captcha     *application.CaptchaService
	Settings    *application.SettingsService
	Moderation  *application.ModerationService
	Invite      *application.InviteService
	Reaction    *application.ReactionService
	Summary     *application.SummaryService
	Zombie      *application.ZombieService
	Fun         *application.FunService
	Pipeline    *application.GroupMessagePipeline
	BotUsername string // without the leading "@"
	RNG         *rand.Rand
}

// Handler dispatches Telegram updates to the application services.
type Handler struct {
	d   Deps
	rng *rand.Rand
}

// NewHandler builds the handler from its dependencies.
func NewHandler(d Deps) *Handler {
	rng := d.RNG
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return &Handler{d: d, rng: rng}
}

// HandleUpdate routes one update. Processing failures are returned so the
// transport layer can log them; the webhook always answers 200 regardless.
func (h *Handler) HandleUpdate(ctx context.Context, u *models.Update) error {
	if u == nil {
		return nil
	}
	switch {
	case u.CallbackQuery != nil:
		return h.handleCallback(ctx, u.CallbackQuery)
	case u.Message != nil:
		return h.handleMessage(ctx, u.Message)
	}
	return nil
}

// --- messages ---------------------------------------------------------------

func (h *Handler) handleMessage(ctx context.Context, m *models.Message) error {
	if len(m.NewChatMembers) > 0 {
		return h.handleNewMembers(ctx, m)
	}
	if name, args, ok := parseCommand(m.Text); ok {
		return h.handleCommand(ctx, m, name, args)
	}
	if m.Text != "" && m.From != nil && isGroupChat(m.Chat) {
		if err := h.d.Pipeline.HandleMessage(ctx, m.Chat.ID, m.From.ID, m.ID, displayName(m.From), m.Text); err != nil {
			log.Printf("pipeline: chat %d: %v", m.Chat.ID, err)
		}
	}
	return nil
}

// handleNewMembers verifies (or welcomes) every member in a join service
// message. Members are processed independently: one failure is logged and
// never blocks the others.
func (h *Handler) handleNewMembers(ctx context.Context, m *models.Message) error {
	for i := range m.NewChatMembers {
		member := &m.NewChatMembers[i]
		if member.IsBot {
			continue
		}
		if err := h.verifyMember(ctx, m.Chat, member); err != nil {
			log.Printf("member join: chat %d user %d: %v", m.Chat.ID, member.ID, err)
		}
	}
	return nil
}

// verifyMember runs one joining member through the captcha flow, or sends
// the welcome message when verification is disabled.
func (h *Handler) verifyMember(ctx context.Context, c models.Chat, member *models.User) error {
	res, err := h.d.Captcha.OnMemberJoined(ctx, c.ID, member.ID, displayName(member))
	if err != nil {
		return err
	}
	if !res.Enabled {
		return h.maybeSendWelcome(ctx, c, member)
	}

	name := displayName(member)
	buttons := captchaButtons(res.Challenge, member.ID)
	if len(res.ImagePNG) > 0 {
		if err := h.d.Telegram.SendPhoto(ctx, c.ID, res.ImagePNG,
			fmt.Sprintf(captchaPhotoCaptionT, name), 0); err != nil {
			return err
		}
	}
	messageID, err := h.d.Telegram.SendText(ctx, c.ID,
		fmt.Sprintf(captchaChallengeT, name, res.Challenge.Question), &ports.SendOpts{Buttons: buttons})
	if err != nil {
		return err
	}
	return h.d.Captcha.BindMessageID(ctx, c.ID, member.ID, messageID)
}

// captchaButtons lays out the challenge options as one inline-keyboard row.
func captchaButtons(challenge moderation.Challenge, userID int64) [][]ports.Button {
	row := make([]ports.Button, 0, len(challenge.Options))
	for i, opt := range challenge.Options {
		row = append(row, ports.Button{Text: opt, Data: encodeCaptchaData(userID, i)})
	}
	return [][]ports.Button{row}
}

// maybeSendWelcome posts the chat's greeting when welcome messages are on.
func (h *Handler) maybeSendWelcome(ctx context.Context, c models.Chat, member *models.User) error {
	st, err := h.d.Settings.Get(ctx, c.ID, c.Title)
	if err != nil {
		return err
	}
	if !st.Welcome.Enabled || st.Welcome.Text == "" {
		return nil
	}
	text := strings.ReplaceAll(st.Welcome.Text, "{name}", displayName(member))
	text = strings.ReplaceAll(text, "{chat}", c.Title)
	_, err = h.d.Telegram.SendText(ctx, c.ID, text, nil)
	return err
}

// --- callbacks --------------------------------------------------------------

func (h *Handler) handleCallback(ctx context.Context, q *models.CallbackQuery) error {
	msg := q.Message.Message
	if msg == nil {
		return nil
	}
	if userID, idx, ok := parseCaptchaData(q.Data); ok {
		return h.handleCaptchaAnswer(ctx, q, msg, userID, idx)
	}
	if action, ok := parsePanelData(q.Data); ok {
		return h.handlePanelAction(ctx, q, msg, action)
	}
	return nil
}

// handleCaptchaAnswer checks one option tap. Only the joining member may
// answer; anyone else gets a modal nudge.
func (h *Handler) handleCaptchaAnswer(ctx context.Context, q *models.CallbackQuery, msg *models.Message, userID int64, optionIndex int) error {
	if q.From.ID != userID {
		return h.d.Telegram.AnswerCallback(ctx, q.ID, textCaptchaNotYours, true)
	}
	res, err := h.d.Captcha.Solve(ctx, msg.Chat.ID, userID, optionIndex)
	if errors.Is(err, application.ErrNotFound) {
		return h.d.Telegram.AnswerCallback(ctx, q.ID, textCaptchaGone, true)
	}
	if err != nil {
		return err
	}
	switch {
	case res.Expired:
		return h.d.Telegram.AnswerCallback(ctx, q.ID, textCaptchaExpired, true)
	case res.Passed:
		if err := h.d.Telegram.AnswerCallback(ctx, q.ID, textCaptchaPassed, false); err != nil {
			return err
		}
		return h.maybeSendWelcome(ctx, msg.Chat, &q.From)
	default:
		return h.d.Telegram.AnswerCallback(ctx, q.ID, textCaptchaWrong, true)
	}
}

// handlePanelAction applies one admin-panel toggle and redraws the panel.
func (h *Handler) handlePanelAction(ctx context.Context, q *models.CallbackQuery, msg *models.Message, action string) error {
	chatID := msg.Chat.ID
	isAdmin, err := h.d.Telegram.IsAdmin(ctx, chatID, q.From.ID)
	if err != nil {
		return err
	}
	if !isAdmin {
		return h.d.Telegram.AnswerCallback(ctx, q.ID, textErrNotAdmin, true)
	}

	if action != panelActionRefresh {
		if err := h.applyPanelToggle(ctx, chatID, q.From.ID, action); err != nil {
			return h.d.Telegram.AnswerCallback(ctx, q.ID, errorText(err), true)
		}
	}
	if err := h.d.Telegram.AnswerCallback(ctx, q.ID, textPanelUpdated, false); err != nil {
		return err
	}
	st, err := h.d.Settings.Get(ctx, chatID, msg.Chat.Title)
	if err != nil {
		return err
	}
	text, buttons := renderPanel(st)
	return h.d.Telegram.EditText(ctx, chatID, msg.ID, text, buttons)
}

// applyPanelToggle performs the settings mutation behind one panel button.
func (h *Handler) applyPanelToggle(ctx context.Context, chatID, userID int64, action string) error {
	st, err := h.d.Settings.Get(ctx, chatID, "")
	if err != nil {
		return err
	}
	switch action {
	case panelActionCaptcha:
		return h.d.Settings.SetCaptchaEnabled(ctx, chatID, userID, !st.Captcha.Enabled)
	case panelActionFilter:
		return h.d.Moderation.SetFilterEnabled(ctx, chatID, userID, !st.Filter.Enabled)
	case panelActionLLM:
		if !st.AutoReact.LLMEnabled && !h.llmAvailable() {
			return application.ErrLLMNotConfigured
		}
		return h.d.Reaction.SetLLMEnabled(ctx, chatID, userID, !st.AutoReact.LLMEnabled)
	case panelActionSummary:
		if st.Summary.AutoEnabled {
			return h.d.Summary.SetAutoSummary(ctx, chatID, userID, 0)
		}
		if !h.llmAvailable() {
			return application.ErrLLMNotConfigured
		}
		return h.d.Summary.SetAutoSummary(ctx, chatID, userID, st.Summary.IntervalHours)
	}
	return application.ErrInvalidArgument
}

// --- commands ---------------------------------------------------------------

// handleCommand routes one parsed command. Commands addressed at another bot
// ("/cmd@otherbot") are ignored.
func (h *Handler) handleCommand(ctx context.Context, m *models.Message, name, args string) error {
	if target := commandTarget(m.Text); target != "" &&
		!strings.EqualFold(target, h.d.BotUsername) {
		return nil
	}
	switch name {
	case "start":
		return h.cmdStart(ctx, m, args)
	case "help":
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textHelp, nil)
		return err
	case "xqt":
		return h.cmdPanel(ctx, m)
	case "invite":
		return h.cmdInvite(ctx, m)
	case "filter":
		return h.cmdFilter(ctx, m, args)
	case "captcha":
		return h.cmdCaptcha(ctx, m, args)
	case "kick", "ban", "mute", "unmute":
		return h.cmdModerate(ctx, m, name, args)
	case "autoreact":
		return h.cmdAutoReact(ctx, m, args)
	case "summary":
		return h.cmdSummary(ctx, m, args)
	case "clean":
		return h.cmdClean(ctx, m, args)
	case "welcome":
		return h.cmdWelcome(ctx, m, args)
	case "roll":
		return h.cmdRoll(ctx, m)
	case "pick":
		return h.cmdPick(ctx, m, args)
	}
	return nil
}

// cmdStart handles /start in private chats: a deep-link payload resolves to
// a one-time invite link, a bare /start explains the bot.
func (h *Handler) cmdStart(ctx context.Context, m *models.Message, payload string) error {
	if m.Chat.Type != models.ChatTypePrivate {
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textStartIntro, nil)
		return err
	}
	if payload == "" {
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textStartIntro, nil)
		return err
	}
	res, err := h.d.Invite.HandleStart(ctx, m.From.ID, payload)
	if err != nil {
		if errors.Is(err, application.ErrInvalidPayload) {
			_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textStartPayloadInvalid, nil)
			return serr
		}
		return err
	}
	buttons := [][]ports.Button{{
		{Text: fmt.Sprintf(inviteJoinButtonT, res.ChatTitle), URL: res.URL},
	}}
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
		fmt.Sprintf(inviteReadyT, res.ChatTitle, res.ExpireMinutes), &ports.SendOpts{Buttons: buttons})
	return err
}

// cmdPanel opens the admin panel (/xqt).
func (h *Handler) cmdPanel(ctx context.Context, m *models.Message) error {
	if !isGroupChat(m.Chat) {
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textGroupOnly, nil)
		return err
	}
	isAdmin, err := h.d.Telegram.IsAdmin(ctx, m.Chat.ID, m.From.ID)
	if err != nil {
		return err
	}
	if !isAdmin {
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textErrNotAdmin, nil)
		return err
	}
	st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
	if err != nil {
		return err
	}
	text, buttons := renderPanel(st)
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, text, &ports.SendOpts{Buttons: buttons})
	return err
}

// renderPanel builds the panel text and toggle keyboard from the settings.
func renderPanel(st *chat.Settings) (string, [][]ports.Button) {
	captchaState := textOff
	if st.Captcha.Enabled {
		captchaState = fmt.Sprintf(panelCaptchaOnT, captchaModeLabel(st.Captcha.Mode), st.Captcha.TimeoutSeconds)
	}
	filterState := textOff
	if st.Filter.Enabled {
		action := filterActionDeleteT
		if st.Filter.MuteMinutes > 0 {
			action = fmt.Sprintf(filterActionMuteT, st.Filter.MuteMinutes)
		}
		filterState = fmt.Sprintf(panelFilterOnT, action)
	}
	summaryState := textOff
	if st.Summary.AutoEnabled {
		summaryState = fmt.Sprintf(panelSummaryOnT, st.Summary.IntervalHours)
	}
	welcomeState := textOff
	if st.Welcome.Enabled {
		welcomeState = textOn
	}
	llmState := textOff
	if st.AutoReact.LLMEnabled {
		llmState = textOn
	}

	text := fmt.Sprintf(panelTextT,
		st.Title,
		captchaState,
		filterState, len(st.Filter.Rules),
		len(st.AutoReact.Rules), llmState,
		summaryState,
		welcomeState,
		st.Zombie.InactiveDays,
		st.Invite.ExpireMinutes,
	)

	buttons := [][]ports.Button{
		{
			{Text: panelButton(st.Captcha.Enabled, "进群验证"), Data: panelCallbackPrefix + panelActionCaptcha},
			{Text: panelButton(st.Filter.Enabled, "敏感词过滤"), Data: panelCallbackPrefix + panelActionFilter},
		},
		{
			{Text: panelButton(st.AutoReact.LLMEnabled, "AI 表情"), Data: panelCallbackPrefix + panelActionLLM},
			{Text: panelButton(st.Summary.AutoEnabled, "自动总结"), Data: panelCallbackPrefix + panelActionSummary},
		},
		{
			{Text: textPanelRefresh, Data: panelCallbackPrefix + panelActionRefresh},
		},
	}
	return text, buttons
}

// panelButton renders one toggle button label with a state emoji.
func panelButton(enabled bool, label string) string {
	state := "❌"
	if enabled {
		state = "✅"
	}
	return fmt.Sprintf(panelButtonT, state, label)
}

// cmdInvite generates the shareable deep link (/invite, admin only).
func (h *Handler) cmdInvite(ctx context.Context, m *models.Message) error {
	link, err := h.d.Invite.CreateShareLink(ctx, m.Chat.ID, m.From.ID)
	if err != nil {
		return h.replyError(ctx, m.Chat.ID, err)
	}
	st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
	if err != nil {
		return err
	}
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
		fmt.Sprintf(inviteShareT, link, st.Invite.ExpireMinutes),
		&ports.SendOpts{DisableLinkPreview: true})
	return err
}

// cmdFilter manages sensitive-word rules (/filter).
func (h *Handler) cmdFilter(ctx context.Context, m *models.Message, args string) error {
	switch {
	case args == "":
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return err
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, filterListText(st), nil)
		return err
	case strings.HasPrefix(args, "add "):
		pattern, isRegex, ok := splitPatternArg(strings.TrimPrefix(args, "add "))
		if !ok {
			_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageFilter, nil)
			return err
		}
		var err error
		if isRegex {
			err = h.d.Moderation.AddRegexRule(ctx, m.Chat.ID, m.From.ID, pattern)
		} else {
			err = h.d.Moderation.AddWordRule(ctx, m.Chat.ID, m.From.ID, pattern)
		}
		if err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textFilterAddedT, pattern), nil)
		return err
	case strings.HasPrefix(args, "del "):
		target := strings.TrimSpace(strings.TrimPrefix(args, "del "))
		pattern, err := h.resolveFilterTarget(ctx, m, target)
		if err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		if err := h.d.Moderation.RemoveRule(ctx, m.Chat.ID, m.From.ID, pattern); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textFilterDeletedT, pattern), nil)
		return err
	default:
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageFilter, nil)
		return err
	}
}

// resolveFilterTarget turns a /filter del argument (rule number or pattern)
// into the stored pattern.
func (h *Handler) resolveFilterTarget(ctx context.Context, m *models.Message, target string) (string, error) {
	if target == "" {
		return "", application.ErrInvalidArgument
	}
	if n, err := strconv.Atoi(target); err == nil {
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return "", err
		}
		if n < 1 || n > len(st.Filter.Rules) {
			return "", application.ErrNotFound
		}
		return st.Filter.Rules[n-1].Pattern, nil
	}
	pattern, _, ok := splitPatternArg(target)
	if !ok {
		return "", application.ErrInvalidArgument
	}
	return pattern, nil
}

// filterListText renders the numbered rule list for /filter with no args.
func filterListText(st *chat.Settings) string {
	state := textOff
	action := filterActionDeleteT
	if st.Filter.MuteMinutes > 0 {
		action = fmt.Sprintf(filterActionMuteT, st.Filter.MuteMinutes)
	}
	if st.Filter.Enabled {
		state = textOn
	}
	var b strings.Builder
	fmt.Fprintf(&b, filterListHeaderT, state, action)
	if len(st.Filter.Rules) == 0 {
		b.WriteString("\n" + textFilterNoRules)
	}
	for i, r := range st.Filter.Rules {
		if r.Kind == moderation.RuleRegex {
			fmt.Fprintf(&b, "\n"+filterRuleLineRegexT, i+1, r.Pattern)
		} else {
			fmt.Fprintf(&b, "\n"+filterRuleLineWordT, i+1, r.Pattern)
		}
	}
	return b.String()
}

// cmdCaptcha configures join verification (/captcha on|off|button|image).
func (h *Handler) cmdCaptcha(ctx context.Context, m *models.Message, args string) error {
	arg := strings.ToLower(strings.TrimSpace(args))
	switch arg {
	case "on", "off":
		if err := h.d.Settings.SetCaptchaEnabled(ctx, m.Chat.ID, m.From.ID, arg == "on"); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return err
		}
		state := "关闭"
		if arg == "on" {
			state = "开启"
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
			fmt.Sprintf(textCaptchaSetT, state, captchaModeLabel(st.Captcha.Mode)), nil)
		return err
	case "button", "image":
		mode := chat.CaptchaModeButton
		if arg == "image" {
			mode = chat.CaptchaModeImage
		}
		if err := h.d.Settings.SetCaptchaMode(ctx, m.Chat.ID, m.From.ID, mode); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID,
			fmt.Sprintf(textCaptchaModeSetT, captchaModeLabel(mode)), nil)
		return err
	default:
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageCaptcha, nil)
		return err
	}
}

// cmdModerate runs the reply-target commands /kick /ban /mute /unmute.
func (h *Handler) cmdModerate(ctx context.Context, m *models.Message, name, args string) error {
	if m.ReplyToMessage == nil || m.ReplyToMessage.From == nil {
		usage := textUsageReply
		if name == "mute" {
			usage = textUsageMute
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, usage, nil)
		return err
	}
	target := m.ReplyToMessage.From
	targetName := displayName(target)

	var (
		text string
		err  error
	)
	switch name {
	case "kick":
		err = h.d.Moderation.Kick(ctx, m.Chat.ID, m.From.ID, target.ID)
		text = fmt.Sprintf(textKickDoneT, targetName)
	case "ban":
		err = h.d.Moderation.Ban(ctx, m.Chat.ID, m.From.ID, target.ID)
		text = fmt.Sprintf(textBanDoneT, targetName)
	case "mute":
		minutes, parseErr := parseMinutes(args, defaultMuteMinutes)
		if parseErr != nil {
			_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageMute, nil)
			return serr
		}
		err = h.d.Moderation.Mute(ctx, m.Chat.ID, m.From.ID, target.ID, minutes)
		text = fmt.Sprintf(textMuteDoneT, targetName, minutes)
	case "unmute":
		err = h.d.Moderation.Unmute(ctx, m.Chat.ID, m.From.ID, target.ID)
		text = fmt.Sprintf(textUnmuteDoneT, targetName)
	}
	if err != nil {
		return h.replyError(ctx, m.Chat.ID, err)
	}
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, text, nil)
	return err
}

// parseMinutes reads a positive minute count, falling back to def on empty.
func parseMinutes(args string, def int) (int, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		return def, nil
	}
	n, err := strconv.Atoi(args)
	if err != nil || n <= 0 {
		return 0, application.ErrInvalidArgument
	}
	return n, nil
}

// cmdAutoReact manages emoji-reaction rules (/autoreact).
func (h *Handler) cmdAutoReact(ctx context.Context, m *models.Message, args string) error {
	switch {
	case args == "":
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return err
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, autoReactListText(st), nil)
		return err
	case args == "llm on" || args == "llm off":
		enable := args == "llm on"
		if enable && !h.llmAvailable() {
			_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textErrLLMNotConfigured, nil)
			return err
		}
		if err := h.d.Reaction.SetLLMEnabled(ctx, m.Chat.ID, m.From.ID, enable); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		state := "开启"
		if !enable {
			state = "关闭"
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textAutoReactLLMSetT, state), nil)
		return err
	case strings.HasPrefix(args, "del "):
		pattern, _, ok := splitPatternArg(strings.TrimPrefix(args, "del "))
		if !ok {
			_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageAutoReact, nil)
			return err
		}
		if err := h.d.Reaction.RemoveRule(ctx, m.Chat.ID, m.From.ID, pattern); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textAutoReactDeletedT, pattern), nil)
		return err
	default:
		pattern, emoji, isRegex, ok := parseAutoReactArgs(args)
		if !ok {
			_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageAutoReact, nil)
			return err
		}
		var err error
		if isRegex {
			err = h.d.Reaction.AddRegexRule(ctx, m.Chat.ID, m.From.ID, pattern, emoji)
		} else {
			err = h.d.Reaction.AddKeywordRule(ctx, m.Chat.ID, m.From.ID, pattern, emoji)
		}
		if err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textAutoReactAddedT, pattern, emoji), nil)
		return err
	}
}

// autoReactListText renders the rule list for /autoreact with no args.
func autoReactListText(st *chat.Settings) string {
	llmState := textOff
	if st.AutoReact.LLMEnabled {
		llmState = textOn
	}
	var b strings.Builder
	fmt.Fprintf(&b, autoReactListHeaderT, len(st.AutoReact.Rules), llmState)
	if len(st.AutoReact.Rules) == 0 {
		b.WriteString("\n" + textAutoReactNoRules)
	}
	for i, r := range st.AutoReact.Rules {
		if r.Kind == reaction.KindRegex {
			fmt.Fprintf(&b, "\n"+autoReactRuleLineRegexT, i+1, r.Pattern, r.Emoji)
		} else {
			fmt.Fprintf(&b, "\n"+autoReactRuleLineKeywordT, i+1, r.Pattern, r.Emoji)
		}
	}
	return b.String()
}

// cmdSummary handles /summary, including the auto-summary configuration.
func (h *Handler) cmdSummary(ctx context.Context, m *models.Message, args string) error {
	if strings.HasPrefix(args, "auto") {
		return h.cmdSummaryAuto(ctx, m, strings.TrimSpace(strings.TrimPrefix(args, "auto")))
	}
	hours := 0
	if args != "" {
		n, err := strconv.Atoi(args)
		if err != nil || n <= 0 {
			_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageSummary, nil)
			return serr
		}
		hours = n
	}
	if !h.llmAvailable() {
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textErrLLMNotConfigured, nil)
		return err
	}
	// Generation is slow: post a placeholder and replace it with the result.
	placeholderID, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textSummaryGenerating,
		&ports.SendOpts{ReplyToMessageID: m.ID})
	if err != nil {
		return err
	}
	res, err := h.d.Summary.SummarizeNow(ctx, m.Chat.ID, m.From.ID, hours)
	if err != nil {
		return h.d.Telegram.EditText(ctx, m.Chat.ID, placeholderID, errorText(err), nil)
	}
	return h.d.Telegram.EditText(ctx, m.Chat.ID, placeholderID,
		fmt.Sprintf(summaryResultT, res.Hours, res.MessageCount, res.Text), nil)
}

// cmdSummaryAuto configures the recurring summary (/summary auto N|off).
func (h *Handler) cmdSummaryAuto(ctx context.Context, m *models.Message, args string) error {
	if args == "off" {
		if err := h.d.Summary.SetAutoSummary(ctx, m.Chat.ID, m.From.ID, 0); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textAutoSummaryOff, nil)
		return err
	}
	n, err := strconv.Atoi(args)
	if err != nil || n <= 0 {
		_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageSummary, nil)
		return serr
	}
	if !h.llmAvailable() {
		_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textErrLLMNotConfigured, nil)
		return serr
	}
	if err := h.d.Summary.SetAutoSummary(ctx, m.Chat.ID, m.From.ID, n); err != nil {
		return h.replyError(ctx, m.Chat.ID, err)
	}
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textAutoSummaryOnT, n), nil)
	return err
}

// cmdClean previews or executes zombie-member cleanup (/clean).
func (h *Handler) cmdClean(ctx context.Context, m *models.Message, args string) error {
	switch {
	case args == "":
		preview, err := h.d.Zombie.Preview(ctx, m.Chat.ID, m.From.ID)
		if err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return err
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
			fmt.Sprintf(cleanPreviewT, st.Zombie.InactiveDays, len(preview.UserIDs)), nil)
		return err
	case args == "go":
		res, err := h.d.Zombie.Clean(ctx, m.Chat.ID, m.From.ID)
		if err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
			fmt.Sprintf(cleanDoneT, len(res.Kicked), res.Skipped), nil)
		return err
	case strings.HasPrefix(args, "days "):
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(args, "days ")))
		if err != nil {
			_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageClean, nil)
			return serr
		}
		if err := h.d.Zombie.SetInactiveDays(ctx, m.Chat.ID, m.From.ID, n); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(cleanDaysSetT, n), nil)
		return err
	default:
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsageClean, nil)
		return err
	}
}

// cmdWelcome views or updates the greeting (/welcome).
func (h *Handler) cmdWelcome(ctx context.Context, m *models.Message, args string) error {
	switch {
	case args == "":
		st, err := h.d.Settings.Get(ctx, m.Chat.ID, m.Chat.Title)
		if err != nil {
			return err
		}
		state := textOff
		if st.Welcome.Enabled {
			state = textOn
		}
		_, err = h.d.Telegram.SendText(ctx, m.Chat.ID,
			fmt.Sprintf(welcomeStatusT, state, st.Welcome.Text)+"\n\n"+textUsageWelcome, nil)
		return err
	case args == "on", args == "off":
		if err := h.d.Settings.SetWelcomeEnabled(ctx, m.Chat.ID, m.From.ID, args == "on"); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		state := "开启"
		if args == "off" {
			state = "关闭"
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textWelcomeToggledT, state), nil)
		return err
	default:
		if err := h.d.Settings.SetWelcome(ctx, m.Chat.ID, m.From.ID, args); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		if err := h.d.Settings.SetWelcomeEnabled(ctx, m.Chat.ID, m.From.ID, true); err != nil {
			return h.replyError(ctx, m.Chat.ID, err)
		}
		_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(textWelcomeSetT, args), nil)
		return err
	}
}

// cmdRoll rolls player-vs-bot d100s (/roll).
func (h *Handler) cmdRoll(ctx context.Context, m *models.Message) error {
	you, me := h.d.Fun.Roll(h.rng)
	outcome := textRollDraw
	switch {
	case you > me:
		outcome = textRollWin
	case you < me:
		outcome = textRollLose
	}
	_, err := h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(rollResultT, you, me, outcome), nil)
	return err
}

// cmdPick chooses one of the given options (/pick a b c).
func (h *Handler) cmdPick(ctx context.Context, m *models.Message, args string) error {
	choice, err := h.d.Fun.Pick(h.rng, strings.Fields(args))
	if err != nil {
		_, serr := h.d.Telegram.SendText(ctx, m.Chat.ID, textUsagePick, nil)
		return serr
	}
	_, err = h.d.Telegram.SendText(ctx, m.Chat.ID, fmt.Sprintf(pickResultT, choice), nil)
	return err
}

// --- helpers ----------------------------------------------------------------

// replyError maps an application error to a user-facing reply.
func (h *Handler) replyError(ctx context.Context, chatID int64, err error) error {
	_, serr := h.d.Telegram.SendText(ctx, chatID, errorText(err), nil)
	return serr
}

// errorText maps application sentinel errors to user-facing copy; unknown
// errors are logged and produce a generic message.
func errorText(err error) string {
	switch {
	case errors.Is(err, application.ErrNotAdmin):
		return textErrNotAdmin
	case errors.Is(err, application.ErrTargetIsAdmin):
		return textErrTargetIsAdmin
	case errors.Is(err, application.ErrLLMNotConfigured):
		return textErrLLMNotConfigured
	case errors.Is(err, application.ErrTooFewMessages):
		return textErrTooFewMessages
	case errors.Is(err, application.ErrDuplicate):
		return textErrDuplicate
	case errors.Is(err, application.ErrNotFound):
		return textErrNotFound
	case errors.Is(err, application.ErrInvalidPayload):
		return textStartPayloadInvalid
	case errors.Is(err, application.ErrInvalidArgument):
		return textErrInvalidArgument
	default:
		log.Printf("unhandled application error: %v", err)
		return textErrUnknown
	}
}

// llmAvailable reports whether an LLM backend is configured.
func (h *Handler) llmAvailable() bool {
	return h.d.LLM != nil && h.d.LLM.Available()
}

// captchaModeLabel renders the captcha mode for user-facing copy.
func captchaModeLabel(mode chat.CaptchaMode) string {
	if mode == chat.CaptchaModeImage {
		return textCaptchaModeImage
	}
	return textCaptchaModeButton
}

// isGroupChat reports whether the chat is a group or supergroup.
func isGroupChat(c models.Chat) bool {
	return c.Type == models.ChatTypeGroup || c.Type == models.ChatTypeSupergroup
}

// displayName renders a Telegram user for user-facing copy.
func displayName(u *models.User) string {
	if u == nil {
		return "新朋友"
	}
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name != "" {
		return name
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return "新朋友"
}
