package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
)

func setupZombie() (*ZombieService, *fakeSettingsRepo, *fakeActivityRepo, *fakeTelegram) {
	repo := newFakeSettingsRepo()
	activity := newFakeActivityRepo()
	tg := newFakeTelegram()
	svc := NewZombieService(repo, activity, tg)
	svc.now = fixedClock
	return svc, repo, activity, tg
}

func TestZombieTouchAndJoin(t *testing.T) {
	svc, _, activity, _ := setupZombie()
	ctx := context.Background()
	if err := svc.Touch(ctx, -1, 1); err != nil {
		t.Fatalf("Touch: %v", err)
	}
	if err := svc.OnJoin(ctx, -1, 2); err != nil {
		t.Fatalf("OnJoin: %v", err)
	}
	seen, _ := activity.LastSeen(ctx, -1)
	if !seen[1].Equal(fixedNow) || !seen[2].Equal(fixedNow) {
		t.Fatalf("unexpected activity: %+v", seen)
	}
}

func seedZombieChat(t *testing.T, repo *fakeSettingsRepo, activity *fakeActivityRepo, tg *fakeTelegram) {
	t.Helper()
	st := chat.Default(-1, "")
	st.Zombie.InactiveDays = 30
	repo.seed(st)
	ctx := context.Background()
	// user 1: silent for 40 days -> zombie
	_ = activity.Touch(ctx, -1, 1, fixedNow.Add(-40*24*time.Hour))
	// user 2: silent for 40 days but admin -> skipped
	_ = activity.Touch(ctx, -1, 2, fixedNow.Add(-40*24*time.Hour))
	// user 3: active yesterday -> kept
	_ = activity.Touch(ctx, -1, 3, fixedNow.Add(-24*time.Hour))
	tg.setAdmin(-1, 2, true)
	tg.setAdmin(-1, 7, true) // requester
}

func TestZombiePreview(t *testing.T) {
	svc, repo, activity, tg := setupZombie()
	seedZombieChat(t, repo, activity, tg)
	ctx := context.Background()

	if _, err := svc.Preview(ctx, -1, 8); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	preview, err := svc.Preview(ctx, -1, 7)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if len(preview.UserIDs) != 1 || preview.UserIDs[0] != 1 {
		t.Fatalf("want only user 1, got %+v", preview.UserIDs)
	}
}

func TestZombieClean(t *testing.T) {
	svc, repo, activity, tg := setupZombie()
	seedZombieChat(t, repo, activity, tg)
	ctx := context.Background()

	res, err := svc.Clean(ctx, -1, 7)
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	if len(res.Kicked) != 1 || res.Kicked[0] != 1 {
		t.Fatalf("want user 1 kicked, got %+v", res.Kicked)
	}
	if res.Skipped != 1 { // the inactive admin
		t.Fatalf("want 1 skipped, got %d", res.Skipped)
	}
	if len(tg.bans) != 1 || tg.bans[0].userID != 1 || len(tg.unbans) != 1 {
		t.Fatalf("want kick of user 1, got bans=%v unbans=%v", tg.bans, tg.unbans)
	}
	seen, _ := activity.LastSeen(ctx, -1)
	if _, ok := seen[1]; ok {
		t.Fatalf("kicked member must be removed from activity")
	}
	if _, ok := seen[3]; !ok {
		t.Fatalf("active member must be kept")
	}
}

func TestSetInactiveDays(t *testing.T) {
	svc, repo, _, tg := setupZombie()
	tg.setAdmin(-1, 7, true)
	ctx := context.Background()

	for _, days := range []int{0, -3, 366} {
		if err := svc.SetInactiveDays(ctx, -1, 7, days); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("days=%d: want ErrInvalidArgument, got %v", days, err)
		}
	}
	if err := svc.SetInactiveDays(ctx, -1, 8, 30); !errors.Is(err, ErrNotAdmin) {
		t.Fatalf("want ErrNotAdmin, got %v", err)
	}
	if err := svc.SetInactiveDays(ctx, -1, 7, 14); err != nil {
		t.Fatalf("SetInactiveDays: %v", err)
	}
	st, _ := repo.Get(ctx, -1)
	if st.Zombie.InactiveDays != 14 {
		t.Fatalf("want 14 days, got %d", st.Zombie.InactiveDays)
	}
}
