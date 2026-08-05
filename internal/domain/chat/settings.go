// Package chat contains the ChatSettings aggregate: all per-group
// configuration owned by the bot, guarded by group administrators.
package chat

import (
	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/reaction"
)

// CaptchaMode selects how a joining member proves they are human.
type CaptchaMode string

const (
	CaptchaModeButton CaptchaMode = "button" // tap the correct inline-keyboard answer
	CaptchaModeImage  CaptchaMode = "image"  // read an arithmetic image, tap the answer
)

// CaptchaConfig controls new-member verification.
type CaptchaConfig struct {
	Enabled        bool        `json:"enabled"`
	Mode           CaptchaMode `json:"mode"`
	TimeoutSeconds int         `json:"timeout_seconds"` // member is kicked when the challenge expires
}

// FilterConfig controls the sensitive-word / regex moderation pipeline.
type FilterConfig struct {
	Enabled       bool                    `json:"enabled"`
	Rules         []moderation.FilterRule `json:"rules"`
	MuteMinutes   int                     `json:"mute_minutes"`   // how long an offender is muted; 0 = delete only
	DeleteMessage bool                    `json:"delete_message"` // delete the offending message
	// Sources lists the remote word-list URLs imported into Rules.
	Sources []string `json:"sources,omitempty"`
}

// AutoReactConfig controls automatic emoji reactions.
type AutoReactConfig struct {
	Rules      []reaction.Rule `json:"rules"`
	LLMEnabled bool            `json:"llm_enabled"` // let the LLM pick a reaction when no rule matches
}

// SummaryConfig controls chat-summary generation.
type SummaryConfig struct {
	AutoEnabled   bool `json:"auto_enabled"`
	IntervalHours int  `json:"interval_hours"` // auto-summary cadence
	MaxMessages   int  `json:"max_messages"`   // how many recent messages feed the LLM
}

// WelcomeConfig controls the greeting sent to new members.
type WelcomeConfig struct {
	Enabled bool   `json:"enabled"`
	Text    string `json:"text"` // supports {name} and {chat} placeholders
}

// InviteConfig controls one-time invite links handed out via deep links.
type InviteConfig struct {
	ExpireMinutes int `json:"expire_minutes"` // invite-link validity window
}

// ZombieConfig controls inactive-member cleanup.
type ZombieConfig struct {
	InactiveDays int `json:"inactive_days"` // members silent for this long are "zombies"
}

// Settings is the aggregate root for one Telegram chat.
type Settings struct {
	ChatID    int64           `json:"chat_id"`
	Title     string          `json:"title"`
	Captcha   CaptchaConfig   `json:"captcha"`
	Filter    FilterConfig    `json:"filter"`
	AutoReact AutoReactConfig `json:"autoreact"`
	Summary   SummaryConfig   `json:"summary"`
	Welcome   WelcomeConfig   `json:"welcome"`
	Invite    InviteConfig    `json:"invite"`
	Zombie    ZombieConfig    `json:"zombie"`
}

// Default returns sensible out-of-the-box settings for a newly seen chat:
// everything safe is on, everything disruptive is off.
func Default(chatID int64, title string) *Settings {
	return &Settings{
		ChatID: chatID,
		Title:  title,
		Captcha: CaptchaConfig{
			Enabled:        false,
			Mode:           CaptchaModeButton,
			TimeoutSeconds: 120,
		},
		Filter: FilterConfig{
			Enabled:       true,
			Rules:         []moderation.FilterRule{},
			MuteMinutes:   10,
			DeleteMessage: true,
		},
		AutoReact: AutoReactConfig{},
		Summary: SummaryConfig{
			AutoEnabled:   false,
			IntervalHours: 6,
			MaxMessages:   200,
		},
		Welcome: WelcomeConfig{
			Enabled: false,
			Text:    "欢迎 {name} 加入 {chat}！",
		},
		Invite: InviteConfig{ExpireMinutes: 10},
		Zombie: ZombieConfig{InactiveDays: 30},
	}
}
