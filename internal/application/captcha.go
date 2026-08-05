package application

import (
	"context"
	"errors"
	"math/rand"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// defaultCaptchaTimeoutSeconds is used when settings carry no positive timeout.
const defaultCaptchaTimeoutSeconds = 120

// CaptchaResult is the outcome of a new member joining a protected chat.
type CaptchaResult struct {
	Enabled   bool
	Challenge moderation.Challenge
	ImagePNG  []byte // set only in image mode
}

// SolveResult is the outcome of one captcha answer attempt.
type SolveResult struct {
	Resolved bool // session finished (passed or expired), no more attempts
	Passed   bool
	Expired  bool
}

// CaptchaService verifies joining members with an arithmetic challenge and
// kicks those who never solve it in time.
type CaptchaService struct {
	settings ports.SettingsRepository
	captchas ports.CaptchaRepository
	tg       ports.TelegramGateway
	img      ports.ImageRenderer
	rng      *rand.Rand
	now      func() time.Time
}

// NewCaptchaService builds the service.
func NewCaptchaService(settings ports.SettingsRepository, captchas ports.CaptchaRepository, tg ports.TelegramGateway, img ports.ImageRenderer, rng *rand.Rand) *CaptchaService {
	return &CaptchaService{
		settings: settings,
		captchas: captchas,
		tg:       tg,
		img:      img,
		rng:      rng,
		now:      clockNow,
	}
}

// OnMemberJoined mutes the newcomer and opens a captcha session. The
// interfaces layer sends the challenge message (buttons or image) and stores
// its message ID back into the session.
func (s *CaptchaService) OnMemberJoined(ctx context.Context, chatID, userID int64, userName string) (*CaptchaResult, error) {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, err
	}
	if !st.Captcha.Enabled {
		return &CaptchaResult{Enabled: false}, nil
	}
	timeout := st.Captcha.TimeoutSeconds
	if timeout <= 0 {
		timeout = defaultCaptchaTimeoutSeconds
	}
	expiresAt := s.now().Add(time.Duration(timeout) * time.Second)
	if err := s.tg.RestrictMember(ctx, chatID, userID, false, expiresAt); err != nil {
		return nil, err
	}
	challenge := moderation.NewChallenge(s.rng)
	session := &moderation.Session{
		ChatID:    chatID,
		UserID:    userID,
		Challenge: challenge,
		ExpiresAt: expiresAt,
	}
	if err := s.captchas.Save(ctx, session); err != nil {
		return nil, err
	}
	res := &CaptchaResult{Enabled: true, Challenge: challenge}
	if st.Captcha.Mode == chat.CaptchaModeImage {
		png, err := s.img.RenderCaptcha(challenge.Question)
		if err != nil {
			return nil, err
		}
		res.ImagePNG = png
	}
	return res, nil
}

// BindMessageID records the challenge message ID into the pending session so
// it can be deleted once the captcha resolves. The interfaces layer calls it
// right after sending the challenge message.
func (s *CaptchaService) BindMessageID(ctx context.Context, chatID, userID int64, messageID int) error {
	session, err := s.captchas.Get(ctx, chatID, userID)
	if err != nil {
		return err
	}
	session.MessageID = messageID
	return s.captchas.Save(ctx, session)
}

// Solve checks one answer attempt against the pending session.
func (s *CaptchaService) Solve(ctx context.Context, chatID, userID int64, optionIndex int) (*SolveResult, error) {
	session, err := s.captchas.Get(ctx, chatID, userID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	now := s.now()
	if session.Expired(now) {
		if err := s.kick(ctx, chatID, userID); err != nil {
			return nil, err
		}
		if err := s.deleteCaptchaMessage(ctx, session); err != nil {
			return nil, err
		}
		if err := s.captchas.Delete(ctx, chatID, userID); err != nil {
			return nil, err
		}
		return &SolveResult{Resolved: true, Passed: false, Expired: true}, nil
	}
	if !session.Correct(optionIndex) {
		return &SolveResult{Resolved: false, Passed: false}, nil
	}
	if err := s.captchas.Delete(ctx, chatID, userID); err != nil {
		return nil, err
	}
	if err := s.tg.RestrictMember(ctx, chatID, userID, true, now); err != nil {
		return nil, err
	}
	if err := s.deleteCaptchaMessage(ctx, session); err != nil {
		return nil, err
	}
	return &SolveResult{Resolved: true, Passed: true}, nil
}

// SweepExpired kicks every member whose captcha session timed out and returns
// how many sessions were processed. Individual failures are skipped so one
// bad session cannot block the sweep.
func (s *CaptchaService) SweepExpired(ctx context.Context, now time.Time) (int, error) {
	sessions, err := s.captchas.ListExpired(ctx, now)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range sessions {
		session := sessions[i]
		if err := s.kick(ctx, session.ChatID, session.UserID); err != nil {
			continue
		}
		// Deleting the captcha message is best-effort: it may already be gone.
		_ = s.deleteCaptchaMessage(ctx, &session)
		if err := s.captchas.Delete(ctx, session.ChatID, session.UserID); err != nil {
			continue
		}
		processed++
	}
	return processed, nil
}

// kick removes a member without banning them (ban + immediate unban).
func (s *CaptchaService) kick(ctx context.Context, chatID, userID int64) error {
	if err := s.tg.BanMember(ctx, chatID, userID, false); err != nil {
		return err
	}
	return s.tg.UnbanMember(ctx, chatID, userID)
}

// deleteCaptchaMessage removes the challenge message when its ID is known.
func (s *CaptchaService) deleteCaptchaMessage(ctx context.Context, session *moderation.Session) error {
	if session.MessageID == 0 {
		return nil
	}
	return s.tg.DeleteMessage(ctx, session.ChatID, session.MessageID)
}
