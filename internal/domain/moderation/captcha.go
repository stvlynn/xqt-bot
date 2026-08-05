package moderation

import (
	"fmt"
	"math/rand"
	"time"
)

// Challenge is one captcha question presented to a joining member.
type Challenge struct {
	Question    string   `json:"question"`     // human-readable prompt, e.g. "3 + 4 = ?"
	Options     []string `json:"options"`      // answer buttons
	AnswerIndex int      `json:"answer_index"` // index of the correct option
}

// NewChallenge generates a simple arithmetic challenge with shuffled options.
// Arithmetic keeps the check language-neutral and trivially renderable both
// as text buttons and as an image.
func NewChallenge(rng *rand.Rand) Challenge {
	a, b := rng.Intn(8)+1, rng.Intn(8)+1
	answer := a + b
	optionSet := map[int]bool{answer: true}
	options := []int{answer}
	for len(options) < 4 {
		candidate := answer + rng.Intn(9) - 4 // near-miss distractors
		if candidate >= 0 && !optionSet[candidate] {
			optionSet[candidate] = true
			options = append(options, candidate)
		}
	}
	rng.Shuffle(len(options), func(i, j int) { options[i], options[j] = options[j], options[i] })
	answerIndex := 0
	strs := make([]string, len(options))
	for i, o := range options {
		if o == answer {
			answerIndex = i
		}
		strs[i] = fmt.Sprintf("%d", o)
	}
	return Challenge{
		Question:    fmt.Sprintf("%d + %d = ?", a, b),
		Options:     strs,
		AnswerIndex: answerIndex,
	}
}

// Session is a pending captcha for one joining member.
type Session struct {
	ChatID    int64     `json:"chat_id"`
	UserID    int64     `json:"user_id"`
	MessageID int       `json:"message_id"` // the captcha message, deleted after resolution
	Challenge Challenge `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Expired reports whether the session timed out.
func (s Session) Expired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// Correct reports whether the given option index solves the challenge.
func (s Session) Correct(optionIndex int) bool {
	return optionIndex == s.Challenge.AnswerIndex
}
