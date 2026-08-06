package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func cm(t models.ChatMemberType, isMember bool) models.ChatMember {
	switch t {
	case models.ChatMemberTypeMember:
		return models.ChatMember{Type: t, Member: &models.ChatMemberMember{User: &models.User{ID: 7}}}
	case models.ChatMemberTypeRestricted:
		return models.ChatMember{Type: t, Restricted: &models.ChatMemberRestricted{User: &models.User{ID: 7}, IsMember: isMember}}
	case models.ChatMemberTypeLeft:
		return models.ChatMember{Type: t, Left: &models.ChatMemberLeft{User: &models.User{ID: 7}}}
	case models.ChatMemberTypeBanned:
		return models.ChatMember{Type: t, Banned: &models.ChatMemberBanned{User: &models.User{ID: 7}}}
	case models.ChatMemberTypeAdministrator:
		return models.ChatMember{Type: t, Administrator: &models.ChatMemberAdministrator{User: models.User{ID: 7}}}
	}
	return models.ChatMember{Type: t}
}

func TestIsJoinTransition(t *testing.T) {
	cases := []struct {
		name     string
		old, new models.ChatMember
		isMember bool
		want     bool
	}{
		{"left->member", cm(models.ChatMemberTypeLeft, false), cm(models.ChatMemberTypeMember, false), false, true},
		{"kicked->member", cm(models.ChatMemberTypeBanned, false), cm(models.ChatMemberTypeMember, false), false, true},
		{"left->restricted(is_member)", cm(models.ChatMemberTypeLeft, false), cm(models.ChatMemberTypeRestricted, true), false, true},
		{"left->restricted(not member)", cm(models.ChatMemberTypeLeft, false), cm(models.ChatMemberTypeRestricted, false), false, false},
		{"member->member (no change)", cm(models.ChatMemberTypeMember, false), cm(models.ChatMemberTypeMember, false), false, false},
		{"member->left (leaving)", cm(models.ChatMemberTypeMember, false), cm(models.ChatMemberTypeLeft, false), false, false},
		{"left->administrator (promoted join? no)", cm(models.ChatMemberTypeLeft, false), cm(models.ChatMemberTypeAdministrator, false), false, false},
	}
	for _, c := range cases {
		if got := isJoinTransition(c.old, c.new); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMemberUser(t *testing.T) {
	for _, tp := range []models.ChatMemberType{
		models.ChatMemberTypeMember, models.ChatMemberTypeRestricted,
		models.ChatMemberTypeLeft, models.ChatMemberTypeBanned,
		models.ChatMemberTypeAdministrator,
	} {
		u := memberUser(cm(tp, true))
		if u == nil || u.ID != 7 {
			t.Errorf("type %s: want user 7, got %+v", tp, u)
		}
	}
}

func TestMarkRecentJoinDedup(t *testing.T) {
	h := NewHandler(Deps{})
	if !h.markRecentJoin(1, 100) {
		t.Fatal("first join should be new")
	}
	if h.markRecentJoin(1, 100) {
		t.Fatal("second join within window should be duplicate")
	}
	if !h.markRecentJoin(1, 101) {
		t.Fatal("different user should be new")
	}
	if !h.markRecentJoin(2, 100) {
		t.Fatal("different chat should be new")
	}
}
