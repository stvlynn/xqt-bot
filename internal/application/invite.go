package application

import (
	"context"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/invite"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// inviteMemberLimit caps every generated link at a single join.
const inviteMemberLimit = 1

// InviteResult is the outcome of resolving a /start join deep link.
type InviteResult struct {
	ChatID        int64
	ChatTitle     string
	URL           string
	ExpireMinutes int
}

// InviteService hands out one-time, short-lived group invite links through
// /start deep links.
type InviteService struct {
	settings    ports.SettingsRepository
	tg          ports.TelegramGateway
	botUsername string
	now         func() time.Time
}

// NewInviteService builds the service. botUsername is the bot's Telegram
// username without the leading "@", used to compose share links.
func NewInviteService(settings ports.SettingsRepository, tg ports.TelegramGateway, botUsername string) *InviteService {
	return &InviteService{
		settings:    settings,
		tg:          tg,
		botUsername: botUsername,
		now:         clockNow,
	}
}

// HandleStart resolves a /start deep-link payload into a fresh one-time
// invite link for the encoded chat.
func (s *InviteService) HandleStart(ctx context.Context, userID int64, payload string) (*InviteResult, error) {
	chatID, err := invite.ParsePayload(payload)
	if err != nil {
		return nil, ErrInvalidPayload
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	expireAt := s.now().Add(time.Duration(st.Invite.ExpireMinutes) * time.Minute)
	url, err := s.tg.CreateInviteLink(ctx, chatID, expireAt, inviteMemberLimit)
	if err != nil {
		return nil, err
	}
	return &InviteResult{
		ChatID:        chatID,
		ChatTitle:     st.Title,
		URL:           url,
		ExpireMinutes: st.Invite.ExpireMinutes,
	}, nil
}

// CreateShareLink returns the deep link an admin can distribute so that
// members receive invite links via HandleStart.
func (s *InviteService) CreateShareLink(ctx context.Context, chatID, requesterID int64) (string, error) {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return "", err
	}
	return "https://t.me/" + s.botUsername + "?start=" + invite.EncodePayload(chatID), nil
}
