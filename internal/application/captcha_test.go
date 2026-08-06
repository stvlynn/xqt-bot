package application

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/chat"
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
)

func setupCaptcha() (*CaptchaService, *fakeSettingsRepo, *fakeCaptchaRepo, *fakeTelegram, *fakeRenderer) {
	repo := newFakeSettingsRepo()
	captchas := newFakeCaptchaRepo()
	tg := newFakeTelegram()
	img := &fakeRenderer{png: []byte("png-bytes")}
	svc := NewCaptchaService(repo, captchas, tg, img, rand.New(rand.NewSource(1)))
	svc.now = fixedClock
	return svc, repo, captchas, tg, img
}

func enableCaptcha(repo *fakeSettingsRepo, mode chat.CaptchaMode) {
	st := chat.Default(-1, "")
	st.Captcha.Enabled = true
	st.Captcha.Mode = mode
	st.Captcha.TimeoutSeconds = 120
	repo.seed(st)
}

func TestOnMemberJoinedDisabled(t *testing.T) {
	svc, _, captchas, tg, _ := setupCaptcha()

	res, err := svc.OnMemberJoined(context.Background(), -1, 42, "newbie")
	if err != nil {
		t.Fatalf("OnMemberJoined: %v", err)
	}
	if res.Enabled {
		t.Fatalf("want disabled result")
	}
	if len(tg.restrictions) != 0 || len(captchas.data) != 0 {
		t.Fatalf("want no restriction and no session")
	}
}

func TestOnMemberJoinedButtonMode(t *testing.T) {
	svc, repo, captchas, tg, _ := setupCaptcha()
	enableCaptcha(repo, chat.CaptchaModeButton)

	res, err := svc.OnMemberJoined(context.Background(), -1, 42, "newbie")
	if err != nil {
		t.Fatalf("OnMemberJoined: %v", err)
	}
	if !res.Enabled || res.ImagePNG != nil {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Challenge.Question == "" || len(res.Challenge.Options) != 4 {
		t.Fatalf("bad challenge: %+v", res.Challenge)
	}
	if len(tg.restrictions) != 1 || tg.restrictions[0].canSend {
		t.Fatalf("want newcomer muted, got %+v", tg.restrictions)
	}
	// The mute is permanent (zero until); it lifts only after the challenge
	// is solved, while the session expiry still governs the kick deadline.
	if !tg.restrictions[0].until.IsZero() {
		t.Fatalf("want permanent mute, got until %v", tg.restrictions[0].until)
	}
	session, err := captchas.Get(context.Background(), -1, 42)
	if err != nil {
		t.Fatalf("session not stored: %v", err)
	}
	if !session.ExpiresAt.Equal(fixedNow.Add(120 * time.Second)) {
		t.Fatalf("unexpected expiry: %v", session.ExpiresAt)
	}
}

func TestOnMemberJoinedImageMode(t *testing.T) {
	svc, repo, _, _, _ := setupCaptcha()
	enableCaptcha(repo, chat.CaptchaModeImage)

	res, err := svc.OnMemberJoined(context.Background(), -1, 42, "newbie")
	if err != nil {
		t.Fatalf("OnMemberJoined: %v", err)
	}
	if string(res.ImagePNG) != "png-bytes" {
		t.Fatalf("want rendered PNG, got %q", res.ImagePNG)
	}
}

func TestSolveNotFound(t *testing.T) {
	svc, _, _, _, _ := setupCaptcha()
	if _, err := svc.Solve(context.Background(), -1, 42, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestSolveCorrect(t *testing.T) {
	svc, repo, captchas, tg, _ := setupCaptcha()
	enableCaptcha(repo, chat.CaptchaModeButton)
	res, err := svc.OnMemberJoined(context.Background(), -1, 42, "newbie")
	if err != nil {
		t.Fatalf("OnMemberJoined: %v", err)
	}
	// Bind the captcha message ID like the interfaces layer would.
	session, _ := captchas.Get(context.Background(), -1, 42)
	session.MessageID = 555
	_ = captchas.Save(context.Background(), session)

	solved, err := svc.Solve(context.Background(), -1, 42, res.Challenge.AnswerIndex)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if !solved.Resolved || !solved.Passed || solved.Expired {
		t.Fatalf("unexpected result: %+v", solved)
	}
	if _, err := captchas.Get(context.Background(), -1, 42); err == nil {
		t.Fatalf("want session deleted")
	}
	if len(tg.restrictions) != 2 || !tg.restrictions[1].canSend {
		t.Fatalf("want un-mute, got %+v", tg.restrictions)
	}
	if len(tg.deleted) != 1 || tg.deleted[0].messageID != 555 {
		t.Fatalf("want captcha message deleted, got %+v", tg.deleted)
	}
}

func TestSolveWrongAnswerKeepsSession(t *testing.T) {
	svc, repo, captchas, _, _ := setupCaptcha()
	enableCaptcha(repo, chat.CaptchaModeButton)
	res, err := svc.OnMemberJoined(context.Background(), -1, 42, "newbie")
	if err != nil {
		t.Fatalf("OnMemberJoined: %v", err)
	}
	wrong := (res.Challenge.AnswerIndex + 1) % len(res.Challenge.Options)
	solved, err := svc.Solve(context.Background(), -1, 42, wrong)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if solved.Resolved || solved.Passed {
		t.Fatalf("want unresolved failed attempt, got %+v", solved)
	}
	if _, err := captchas.Get(context.Background(), -1, 42); err != nil {
		t.Fatalf("session must survive a wrong answer: %v", err)
	}
}

func TestSolveExpiredKicks(t *testing.T) {
	svc, _, captchas, tg, _ := setupCaptcha()
	session := &moderation.Session{
		ChatID:    -1,
		UserID:    42,
		MessageID: 555,
		Challenge: moderation.NewChallenge(rand.New(rand.NewSource(2))),
		ExpiresAt: fixedNow.Add(-time.Second), // already expired
	}
	_ = captchas.Save(context.Background(), session)

	res, err := svc.Solve(context.Background(), -1, 42, 0)
	if err != nil {
		t.Fatalf("Solve: %v", err)
	}
	if !res.Resolved || res.Passed || !res.Expired {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(tg.bans) != 1 || len(tg.unbans) != 1 {
		t.Fatalf("want kick (ban+unban), got bans=%v unbans=%v", tg.bans, tg.unbans)
	}
	if len(tg.deleted) != 1 {
		t.Fatalf("want captcha message deleted, got %+v", tg.deleted)
	}
	if _, err := captchas.Get(context.Background(), -1, 42); err == nil {
		t.Fatalf("want session deleted")
	}
}

func TestSweepExpired(t *testing.T) {
	svc, _, captchas, tg, _ := setupCaptcha()
	expired := &moderation.Session{
		ChatID:    -1,
		UserID:    1,
		MessageID: 10,
		Challenge: moderation.NewChallenge(rand.New(rand.NewSource(3))),
		ExpiresAt: fixedNow.Add(-time.Minute),
	}
	pending := &moderation.Session{
		ChatID:    -1,
		UserID:    2,
		Challenge: moderation.NewChallenge(rand.New(rand.NewSource(4))),
		ExpiresAt: fixedNow.Add(time.Minute),
	}
	_ = captchas.Save(context.Background(), expired)
	_ = captchas.Save(context.Background(), pending)

	n, err := svc.SweepExpired(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("SweepExpired: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 processed, got %d", n)
	}
	if len(tg.bans) != 1 || tg.bans[0].userID != 1 || len(tg.unbans) != 1 {
		t.Fatalf("want user 1 kicked, got bans=%v unbans=%v", tg.bans, tg.unbans)
	}
	if len(tg.deleted) != 1 || tg.deleted[0].messageID != 10 {
		t.Fatalf("want captcha message deleted, got %+v", tg.deleted)
	}
	if _, err := captchas.Get(context.Background(), -1, 1); err == nil {
		t.Fatalf("want expired session deleted")
	}
	if _, err := captchas.Get(context.Background(), -1, 2); err != nil {
		t.Fatalf("pending session must survive: %v", err)
	}
}

func TestCaptchaDuplicateJoinIsPending(t *testing.T) {
	settings := newFakeSettingsRepo()
	st := chat.Default(-1001, "g")
	st.Captcha.Enabled = true
	settings.seed(st)

	captchas := newFakeCaptchaRepo()
	tg := newFakeTelegram()
	svc := NewCaptchaService(settings, captchas, tg, &fakeRenderer{}, rand.New(rand.NewSource(1)))
	svc.now = fixedClock

	first, err := svc.OnMemberJoined(context.Background(), -1001, 42, "u")
	if err != nil {
		t.Fatal(err)
	}
	if !first.Enabled || first.Pending {
		t.Fatalf("first join should open a session: %+v", first)
	}
	second, err := svc.OnMemberJoined(context.Background(), -1001, 42, "u")
	if err != nil {
		t.Fatal(err)
	}
	if !second.Pending {
		t.Fatalf("duplicate join should report Pending: %+v", second)
	}
	if n := len(tg.restrictions); n != 1 {
		t.Fatalf("duplicate join must not re-restrict, got %d restrictions", n)
	}
}

func TestCaptchaJoinRestrictsPermanently(t *testing.T) {
	settings := newFakeSettingsRepo()
	st := chat.Default(-1001, "g")
	st.Captcha.Enabled = true
	settings.seed(st)

	tg := newFakeTelegram()
	svc := NewCaptchaService(settings, newFakeCaptchaRepo(), tg, &fakeRenderer{}, rand.New(rand.NewSource(1)))
	svc.now = fixedClock

	if _, err := svc.OnMemberJoined(context.Background(), -1001, 42, "u"); err != nil {
		t.Fatal(err)
	}
	if got := tg.restrictions[0].until; !got.IsZero() {
		t.Fatalf("restriction should be permanent (zero until), got %v", got)
	}
}
