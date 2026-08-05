package kv

import (
	"context"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
)

func testSession(chatID, userID int64, expiresAt time.Time) *moderation.Session {
	return &moderation.Session{
		ChatID: chatID,
		UserID: userID,
		Challenge: moderation.Challenge{
			Question:    "3 + 4 = ?",
			Options:     []string{"7", "5", "9", "11"},
			AnswerIndex: 0,
		},
		ExpiresAt: expiresAt,
	}
}

func TestCaptchaKey(t *testing.T) {
	if got := CaptchaKey(-1001, 55); got != "captcha:-1001:55" {
		t.Fatalf("CaptchaKey = %q", got)
	}
}

func TestCaptchaRoundTrip(t *testing.T) {
	repo := NewCaptchaRepository(NewMemoryStore())
	ctx := context.Background()
	s := testSession(1, 2, time.Now().Add(2*time.Minute))
	if err := repo.Save(ctx, s); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := repo.Get(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Challenge.Question != "3 + 4 = ?" || !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("round trip mismatch: %+v", got)
	}
	if err := repo.Delete(ctx, 1, 2); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, 1, 2); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestCaptchaListExpired(t *testing.T) {
	repo := NewCaptchaRepository(NewMemoryStore())
	ctx := context.Background()
	now := time.Now()
	expired := testSession(1, 1, now.Add(-time.Minute))
	pending := testSession(1, 2, now.Add(time.Minute))
	if err := repo.Save(ctx, expired); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, pending); err != nil {
		t.Fatal(err)
	}
	got, err := repo.ListExpired(ctx, now)
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}
	if len(got) != 1 || got[0].UserID != 1 {
		t.Errorf("ListExpired = %+v", got)
	}
}

func TestCaptchaListExpiredCap(t *testing.T) {
	repo := NewCaptchaRepository(NewMemoryStore())
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < listExpiredCap+20; i++ {
		s := testSession(1, int64(1000+i), now.Add(-time.Minute))
		if err := repo.Save(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.ListExpired(ctx, now)
	if err != nil {
		t.Fatalf("ListExpired: %v", err)
	}
	if len(got) != listExpiredCap {
		t.Errorf("ListExpired returned %d, want cap %d", len(got), listExpiredCap)
	}
}

func TestCaptchaSavePastExpiryKeepsMinimumTTL(t *testing.T) {
	// A session already past expiry must still be storable (sweeper seed).
	repo := NewCaptchaRepository(NewMemoryStore())
	s := testSession(3, 4, time.Now().Add(-time.Hour))
	if err := repo.Save(context.Background(), s); err != nil {
		t.Fatalf("Save with past expiry: %v", err)
	}
	if _, err := repo.Get(context.Background(), 3, 4); err != nil {
		t.Fatalf("Get: %v", err)
	}
}
