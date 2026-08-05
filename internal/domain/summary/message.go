// Package summary defines the recorded chat messages used for summaries.
package summary

import "time"

// Message is one recorded group message. Only text-bearing messages are
// stored; the log is a bounded ring buffer per chat.
type Message struct {
	MessageID int       `json:"message_id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"` // display name at record time
	Text      string    `json:"text"`
	At        time.Time `json:"at"`
}

// Ring is a fixed-capacity, oldest-first message buffer.
type Ring struct {
	Capacity int       `json:"capacity"`
	Messages []Message `json:"messages"`
}

// NewRing creates a ring with the given capacity (minimum 1).
func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = 1
	}
	return &Ring{Capacity: capacity}
}

// Append records a message, evicting the oldest when full.
func (r *Ring) Append(m Message) {
	r.Messages = append(r.Messages, m)
	if len(r.Messages) > r.Capacity {
		r.Messages = r.Messages[len(r.Messages)-r.Capacity:]
	}
}

// Since returns messages newer than t.
func (r *Ring) Since(t time.Time) []Message {
	out := make([]Message, 0, len(r.Messages))
	for _, m := range r.Messages {
		if m.At.After(t) {
			out = append(out, m)
		}
	}
	return out
}
