package application

// In-memory fakes for every ports interface used by the services. Tests are
// white-box (same package) so they can inject clocks and inspect recorded
// calls directly.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/channelpost"
	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/schedule"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

var fixedNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }

// --- SettingsRepository ---

type fakeSettingsRepo struct {
	mu      sync.Mutex
	data    map[int64]*chat.Settings
	getErr  error
	saveErr error
}

func newFakeSettingsRepo() *fakeSettingsRepo {
	return &fakeSettingsRepo{data: make(map[int64]*chat.Settings)}
}

func (f *fakeSettingsRepo) Get(_ context.Context, chatID int64) (*chat.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	st, ok := f.data[chatID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := *st
	return &cp, nil
}

func (f *fakeSettingsRepo) Save(_ context.Context, st *chat.Settings) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *st
	f.data[st.ChatID] = &cp
	return nil
}

func (f *fakeSettingsRepo) seed(st *chat.Settings) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[st.ChatID] = st
}

// --- CaptchaRepository ---

type fakeCaptchaRepo struct {
	mu   sync.Mutex
	data map[[2]int64]moderation.Session
}

func newFakeCaptchaRepo() *fakeCaptchaRepo {
	return &fakeCaptchaRepo{data: make(map[[2]int64]moderation.Session)}
}

func (f *fakeCaptchaRepo) Save(_ context.Context, s *moderation.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[[2]int64{s.ChatID, s.UserID}] = *s
	return nil
}

func (f *fakeCaptchaRepo) Get(_ context.Context, chatID, userID int64) (*moderation.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.data[[2]int64{chatID, userID}]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := s
	return &cp, nil
}

func (f *fakeCaptchaRepo) Delete(_ context.Context, chatID, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, [2]int64{chatID, userID})
	return nil
}

func (f *fakeCaptchaRepo) ListExpired(_ context.Context, now time.Time) ([]moderation.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []moderation.Session
	for _, s := range f.data {
		if s.Expired(now) {
			out = append(out, s)
		}
	}
	return out, nil
}

// --- MessageLogRepository ---

type fakeMessageLogRepo struct {
	mu        sync.Mutex
	data      map[int64][]summary.Message
	appendErr error
}

func newFakeMessageLogRepo() *fakeMessageLogRepo {
	return &fakeMessageLogRepo{data: make(map[int64][]summary.Message)}
}

func (f *fakeMessageLogRepo) Append(_ context.Context, chatID int64, m summary.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return f.appendErr
	}
	f.data[chatID] = append(f.data[chatID], m)
	return nil
}

func (f *fakeMessageLogRepo) Recent(_ context.Context, chatID int64) ([]summary.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]summary.Message, len(f.data[chatID]))
	copy(out, f.data[chatID])
	return out, nil
}

// --- ActivityRepository ---

type fakeActivityRepo struct {
	mu   sync.Mutex
	data map[int64]map[int64]time.Time
}

func newFakeActivityRepo() *fakeActivityRepo {
	return &fakeActivityRepo{data: make(map[int64]map[int64]time.Time)}
}

func (f *fakeActivityRepo) Touch(_ context.Context, chatID, userID int64, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.data[chatID] == nil {
		f.data[chatID] = make(map[int64]time.Time)
	}
	f.data[chatID][userID] = at
	return nil
}

func (f *fakeActivityRepo) LastSeen(_ context.Context, chatID int64) (map[int64]time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[int64]time.Time, len(f.data[chatID]))
	for uid, t := range f.data[chatID] {
		out[uid] = t
	}
	return out, nil
}

func (f *fakeActivityRepo) Remove(_ context.Context, chatID, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data[chatID], userID)
	return nil
}

// --- TaskRepository ---

type fakeTaskRepo struct {
	mu   sync.Mutex
	data map[[2]any]schedule.Task // key: (kind, chatID)
}

func newFakeTaskRepo() *fakeTaskRepo {
	return &fakeTaskRepo{data: make(map[[2]any]schedule.Task)}
}

func (f *fakeTaskRepo) List(_ context.Context) ([]schedule.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]schedule.Task, 0, len(f.data))
	for _, t := range f.data {
		out = append(out, t)
	}
	return out, nil
}

func (f *fakeTaskRepo) Save(_ context.Context, t schedule.Task) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[[2]any{t.Kind, t.ChatID}] = t
	return nil
}

func (f *fakeTaskRepo) Delete(_ context.Context, kind schedule.Kind, chatID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, [2]any{kind, chatID})
	return nil
}

// --- TelegramGateway ---

type sentText struct {
	chatID int64
	text   string
}

type deletedMessage struct {
	chatID    int64
	messageID int
}

type reactionSet struct {
	chatID    int64
	messageID int
	emoji     string
}

type restriction struct {
	chatID  int64
	userID  int64
	canSend bool
	until   time.Time
}

type ban struct {
	chatID int64
	userID int64
	revoke bool
}

type inviteCall struct {
	chatID      int64
	expireAt    time.Time
	memberLimit int
}

type copyCall struct {
	fromChatID int64
	toChatID   int64
	messageID  int
	buttons    [][]ports.Button
}

type editButtonsCall struct {
	chatID    int64
	messageID int
	buttons   [][]ports.Button
}

type fakeTelegram struct {
	mu sync.Mutex

	admins      map[[2]int64]bool
	botNotAdmin map[int64]bool // chats where the bot itself lacks admin rights

	texts        []sentText
	deleted      []deletedMessage
	reactions    []reactionSet
	restrictions []restriction
	bans         []ban
	unbans       [][2]int64
	inviteCalls  []inviteCall
	pinned       [][2]int64
	unpinned     [][2]int64
	copies       []copyCall
	buttonEdits  []editButtonsCall

	inviteURL  string
	chatTitle  string                    // returned by ChatTitle
	chatInfos  map[string]ports.ChatInfo // returned by ChatInfo, keyed by fmt.Sprint(chatRef)
	copyNextID int                       // next message ID returned by CopyMessage
	editErr    error                     // injected failure for EditButtons
	err        error                     // injected failure for all mutating calls
}

func newFakeTelegram() *fakeTelegram {
	return &fakeTelegram{
		admins:      make(map[[2]int64]bool),
		botNotAdmin: make(map[int64]bool),
		chatInfos:   make(map[string]ports.ChatInfo),
		inviteURL:   "https://t.me/+fakeinvite",
	}
}

func (f *fakeTelegram) setAdmin(chatID, userID int64, isAdmin bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.admins[[2]int64{chatID, userID}] = isAdmin
}

func (f *fakeTelegram) SendText(_ context.Context, chatID int64, text string, _ *ports.SendOpts) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	f.texts = append(f.texts, sentText{chatID: chatID, text: text})
	return len(f.texts), nil
}

func (f *fakeTelegram) SendPhoto(_ context.Context, _ int64, _ []byte, _ string, _ int) error {
	return f.err
}

func (f *fakeTelegram) EditText(_ context.Context, _ int64, _ int, _ string, _ [][]ports.Button) error {
	return f.err
}

func (f *fakeTelegram) DeleteMessage(_ context.Context, chatID int64, messageID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, deletedMessage{chatID: chatID, messageID: messageID})
	return nil
}

func (f *fakeTelegram) AnswerCallback(_ context.Context, _, _ string, _ bool) error {
	return f.err
}

func (f *fakeTelegram) CreateInviteLink(_ context.Context, chatID int64, expireAt time.Time, memberLimit int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.inviteCalls = append(f.inviteCalls, inviteCall{chatID: chatID, expireAt: expireAt, memberLimit: memberLimit})
	return f.inviteURL, nil
}

func (f *fakeTelegram) RestrictMember(_ context.Context, chatID, userID int64, canSend bool, until time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.restrictions = append(f.restrictions, restriction{chatID: chatID, userID: userID, canSend: canSend, until: until})
	return nil
}

func (f *fakeTelegram) BanMember(_ context.Context, chatID, userID int64, revokeMessages bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.bans = append(f.bans, ban{chatID: chatID, userID: userID, revoke: revokeMessages})
	return nil
}

func (f *fakeTelegram) UnbanMember(_ context.Context, chatID, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.unbans = append(f.unbans, [2]int64{chatID, userID})
	return nil
}

func (f *fakeTelegram) SetReaction(_ context.Context, chatID int64, messageID int, emoji string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.reactions = append(f.reactions, reactionSet{chatID: chatID, messageID: messageID, emoji: emoji})
	return nil
}

func (f *fakeTelegram) PinMessage(_ context.Context, chatID int64, messageID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.pinned = append(f.pinned, [2]int64{chatID, int64(messageID)})
	return nil
}

func (f *fakeTelegram) UnpinMessage(_ context.Context, chatID int64, messageID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.unpinned = append(f.unpinned, [2]int64{chatID, int64(messageID)})
	return nil
}

func (f *fakeTelegram) IsAdmin(_ context.Context, chatID, userID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.admins[[2]int64{chatID, userID}], nil
}

func (f *fakeTelegram) BotIsAdmin(_ context.Context, chatID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.botNotAdmin[chatID], nil
}

func (f *fakeTelegram) ChatTitle(_ context.Context, _ int64) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.chatTitle, nil
}

func (f *fakeTelegram) CopyMessage(_ context.Context, fromChatID, toChatID int64, messageID int, buttons [][]ports.Button) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	f.copies = append(f.copies, copyCall{fromChatID: fromChatID, toChatID: toChatID, messageID: messageID, buttons: buttons})
	f.copyNextID++
	return 1000 + f.copyNextID, nil
}

func (f *fakeTelegram) EditButtons(_ context.Context, chatID int64, messageID int, buttons [][]ports.Button) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.editErr != nil {
		return f.editErr
	}
	f.buttonEdits = append(f.buttonEdits, editButtonsCall{chatID: chatID, messageID: messageID, buttons: buttons})
	return nil
}

func (f *fakeTelegram) ChatInfo(_ context.Context, chatRef any) (*ports.ChatInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := fmt.Sprint(chatRef)
	info, ok := f.chatInfos[key]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := info
	return &cp, nil
}

// --- LLMGateway ---

type fakeLLM struct {
	mu            sync.Mutex
	available     bool
	summaryText   string
	summaryErr    error
	reactionEmoji string
	reactionOK    bool
	reactionErr   error
	summaryCalls  int
	pickCalls     int
}

func (f *fakeLLM) Available() bool { return f.available }

func (f *fakeLLM) Summarize(_ context.Context, _ []summary.Message) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaryCalls++
	if f.summaryErr != nil {
		return "", f.summaryErr
	}
	return f.summaryText, nil
}

func (f *fakeLLM) PickReaction(_ context.Context, _ string, _ []string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pickCalls++
	if f.reactionErr != nil {
		return "", false, f.reactionErr
	}
	return f.reactionEmoji, f.reactionOK, nil
}

// --- ImageRenderer ---

type fakeRenderer struct {
	png []byte
	err error
}

func (f *fakeRenderer) RenderCaptcha(_ string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.png, nil
}

// --- WordListGateway ---

type fakeWordList struct {
	mu    sync.Mutex
	rules map[string][]moderation.FilterRule
	errs  map[string]error
	calls []string
}

func newFakeWordList() *fakeWordList {
	return &fakeWordList{
		rules: make(map[string][]moderation.FilterRule),
		errs:  make(map[string]error),
	}
}

func (f *fakeWordList) Fetch(_ context.Context, url string) ([]moderation.FilterRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, url)
	if err := f.errs[url]; err != nil {
		return nil, err
	}
	rules := f.rules[url]
	out := make([]moderation.FilterRule, len(rules))
	copy(out, rules)
	return out, nil
}

// wordRules builds fetched rules for a URL, tagged with it as the source.
func wordRules(url string, patterns ...string) []moderation.FilterRule {
	rules := make([]moderation.FilterRule, 0, len(patterns))
	for _, p := range patterns {
		rules = append(rules, moderation.FilterRule{Kind: moderation.RuleWord, Pattern: p, Source: url})
	}
	return rules
}

// --- ChannelBindingRepository ---

type fakeChannelBindingRepo struct {
	mu   sync.Mutex
	data map[int64]int64 // channelID -> groupID
}

func newFakeChannelBindingRepo() *fakeChannelBindingRepo {
	return &fakeChannelBindingRepo{data: make(map[int64]int64)}
}

func (f *fakeChannelBindingRepo) Set(_ context.Context, channelID, groupID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[channelID] = groupID
	return nil
}

func (f *fakeChannelBindingRepo) GetByChannel(_ context.Context, channelID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	groupID, ok := f.data[channelID]
	if !ok {
		return 0, ports.ErrNotFound
	}
	return groupID, nil
}

func (f *fakeChannelBindingRepo) Delete(_ context.Context, channelID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, channelID)
	return nil
}

// --- ForwardedPostRepository ---

type fakeForwardedPostRepo struct {
	mu   sync.Mutex
	data map[[2]int64]channelpost.ForwardedPost // key: (channelID, postID)
}

func newFakeForwardedPostRepo() *fakeForwardedPostRepo {
	return &fakeForwardedPostRepo{data: make(map[[2]int64]channelpost.ForwardedPost)}
}

func (f *fakeForwardedPostRepo) Save(_ context.Context, p channelpost.ForwardedPost) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[[2]int64{p.ChannelID, int64(p.PostID)}] = p
	return nil
}

func (f *fakeForwardedPostRepo) Get(_ context.Context, channelID int64, postID int) (*channelpost.ForwardedPost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.data[[2]int64{channelID, int64(postID)}]
	if !ok {
		return nil, ports.ErrNotFound
	}
	cp := p
	return &cp, nil
}

// --- CommentLogRepository ---

type fakeCommentLogRepo struct {
	mu   sync.Mutex
	logs map[[2]int64]channelpost.CommentLog // key: (channelID, postID)
}

func newFakeCommentLogRepo() *fakeCommentLogRepo {
	return &fakeCommentLogRepo{logs: make(map[[2]int64]channelpost.CommentLog)}
}

func (f *fakeCommentLogRepo) Append(_ context.Context, channelID int64, postID int, c channelpost.Comment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [2]int64{channelID, int64(postID)}
	log := f.logs[key]
	log.Add(c)
	f.logs[key] = log
	return nil
}

func (f *fakeCommentLogRepo) List(_ context.Context, channelID int64, postID int) ([]channelpost.Comment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	src := f.logs[[2]int64{channelID, int64(postID)}].Comments
	out := make([]channelpost.Comment, len(src))
	copy(out, src)
	return out, nil
}
