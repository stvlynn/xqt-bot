package application

import (
	"context"
	"sort"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// maxInactiveDays bounds the zombie threshold an admin can configure.
const maxInactiveDays = 365

// ZombiePreview lists the members a cleanup would kick.
type ZombiePreview struct {
	UserIDs []int64
}

// ZombieResult reports what one cleanup run did.
type ZombieResult struct {
	Kicked  []int64
	Skipped int // admins and members Telegram refused to kick
}

// ZombieService tracks member activity and removes members that have been
// silent longer than the chat's configured threshold.
type ZombieService struct {
	settings ports.SettingsRepository
	activity ports.ActivityRepository
	tg       ports.TelegramGateway
	now      func() time.Time
}

// NewZombieService builds the service.
func NewZombieService(settings ports.SettingsRepository, activity ports.ActivityRepository, tg ports.TelegramGateway) *ZombieService {
	return &ZombieService{settings: settings, activity: activity, tg: tg, now: clockNow}
}

// Touch records that a member was active right now.
func (s *ZombieService) Touch(ctx context.Context, chatID, userID int64) error {
	return s.activity.Touch(ctx, chatID, userID, s.now())
}

// OnJoin records a member join as activity.
func (s *ZombieService) OnJoin(ctx context.Context, chatID, userID int64) error {
	return s.Touch(ctx, chatID, userID)
}

// Preview lists the members a cleanup would kick, without touching anyone.
func (s *ZombieService) Preview(ctx context.Context, chatID, requesterID int64) (*ZombiePreview, error) {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return nil, err
	}
	zombies, _, err := s.findZombies(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return &ZombiePreview{UserIDs: zombies}, nil
}

// Clean kicks every inactive non-admin member and forgets their activity.
func (s *ZombieService) Clean(ctx context.Context, chatID, requesterID int64) (*ZombieResult, error) {
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return nil, err
	}
	return s.clean(ctx, chatID)
}

// clean is the admin-check-free core shared with the TaskRunner.
func (s *ZombieService) clean(ctx context.Context, chatID int64) (*ZombieResult, error) {
	zombies, skipped, err := s.findZombies(ctx, chatID)
	if err != nil {
		return nil, err
	}
	res := &ZombieResult{Skipped: skipped}
	for _, userID := range zombies {
		if err := s.tg.BanMember(ctx, chatID, userID, false); err != nil {
			res.Skipped++
			continue
		}
		if err := s.tg.UnbanMember(ctx, chatID, userID); err != nil {
			res.Skipped++
			continue
		}
		if err := s.activity.Remove(ctx, chatID, userID); err != nil {
			res.Skipped++
			continue
		}
		res.Kicked = append(res.Kicked, userID)
	}
	return res, nil
}

// findZombies returns the inactive non-admin members (sorted) and how many
// inactive members were skipped because they are admins.
func (s *ZombieService) findZombies(ctx context.Context, chatID int64) (zombies []int64, skipped int, err error) {
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return nil, 0, err
	}
	seen, err := s.activity.LastSeen(ctx, chatID)
	if err != nil {
		return nil, 0, err
	}
	cutoff := s.now().Add(-time.Duration(st.Zombie.InactiveDays) * 24 * time.Hour)
	for userID, lastSeen := range seen {
		if !lastSeen.Before(cutoff) {
			continue
		}
		isAdmin, err := s.tg.IsAdmin(ctx, chatID, userID)
		if err != nil {
			return nil, 0, err
		}
		if isAdmin {
			skipped++
			continue
		}
		zombies = append(zombies, userID)
	}
	sort.Slice(zombies, func(i, j int) bool { return zombies[i] < zombies[j] })
	return zombies, skipped, nil
}

// SetInactiveDays configures how many silent days make a member a zombie.
func (s *ZombieService) SetInactiveDays(ctx context.Context, chatID, requesterID int64, days int) error {
	if days < 1 || days > maxInactiveDays {
		return ErrInvalidArgument
	}
	if err := requireAdmin(ctx, s.tg, chatID, requesterID); err != nil {
		return err
	}
	st, err := loadSettings(ctx, s.settings, chatID)
	if err != nil {
		return err
	}
	st.Zombie.InactiveDays = days
	return s.settings.Save(ctx, st)
}
