package application

import (
	"context"
	"strings"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// maxInviteExpireMinutes is Telegram's upper bound for invite-link lifetime
// (7 days).
const maxInviteExpireMinutes = 7 * 24 * 60

// SettingsService exposes the admin-facing settings operations that do not
// belong to a more specific service.
type SettingsService struct {
	settings ports.SettingsRepository
	tg       ports.TelegramGateway
}

// NewSettingsService builds the service.
func NewSettingsService(settings ports.SettingsRepository, tg ports.TelegramGateway) *SettingsService {
	return &SettingsService{settings: settings, tg: tg}
}

// Get returns the chat's settings, or defaults (titled with the given name)
// when the chat has never been persisted.
func (s *SettingsService) Get(ctx context.Context, chatID int64, title string) (*chat.Settings, error) {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	if st.Title == "" {
		st.Title = title
	}
	return st, nil
}

// SetCaptchaEnabled toggles join verification.
func (s *SettingsService) SetCaptchaEnabled(ctx context.Context, chatID, requesterID int64, enabled bool) error {
	return s.update(ctx, chatID, requesterID, func(st *chat.Settings) error {
		st.Captcha.Enabled = enabled
		return nil
	})
}

// SetCaptchaMode switches between button and image captchas.
func (s *SettingsService) SetCaptchaMode(ctx context.Context, chatID, requesterID int64, mode chat.CaptchaMode) error {
	if mode != chat.CaptchaModeButton && mode != chat.CaptchaModeImage {
		return ErrInvalidArgument
	}
	return s.update(ctx, chatID, requesterID, func(st *chat.Settings) error {
		st.Captcha.Mode = mode
		return nil
	})
}

// SetWelcome sets the greeting text (supports {name} and {chat} placeholders).
func (s *SettingsService) SetWelcome(ctx context.Context, chatID, requesterID int64, text string) error {
	if strings.TrimSpace(text) == "" {
		return ErrInvalidArgument
	}
	return s.update(ctx, chatID, requesterID, func(st *chat.Settings) error {
		st.Welcome.Text = text
		return nil
	})
}

// SetWelcomeEnabled toggles the new-member greeting.
func (s *SettingsService) SetWelcomeEnabled(ctx context.Context, chatID, requesterID int64, enabled bool) error {
	return s.update(ctx, chatID, requesterID, func(st *chat.Settings) error {
		st.Welcome.Enabled = enabled
		return nil
	})
}

// SetInviteExpireMinutes sets how long generated invite links stay valid.
func (s *SettingsService) SetInviteExpireMinutes(ctx context.Context, chatID, requesterID int64, minutes int) error {
	if minutes < 1 || minutes > maxInviteExpireMinutes {
		return ErrInvalidArgument
	}
	return s.update(ctx, chatID, requesterID, func(st *chat.Settings) error {
		st.Invite.ExpireMinutes = minutes
		return nil
	})
}

// update applies an admin-authorized mutation to the chat's settings.
func (s *SettingsService) update(ctx context.Context, chatID, requesterID int64, mutate func(*chat.Settings) error) error {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	if err := mutate(st); err != nil {
		return err
	}
	return s.settings.Save(ctx, st)
}
