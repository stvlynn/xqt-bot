package ports

import (
	"context"

	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// LLMGateway abstracts the configured large-language-model backend
// (any OpenAI-compatible chat-completions endpoint).
type LLMGateway interface {
	// Available reports whether an LLM endpoint is configured at all.
	Available() bool
	// Summarize produces a concise Chinese summary of the given messages.
	Summarize(ctx context.Context, messages []summary.Message) (string, error)
	// PickReaction chooses one emoji from whitelist for the message,
	// or returns ok=false when the message deserves no reaction.
	PickReaction(ctx context.Context, text string, whitelist []string) (emoji string, ok bool, err error)
}

// ImageRenderer renders PNG images inside the worker.
type ImageRenderer interface {
	// RenderCaptcha draws a captcha challenge prompt (ASCII-safe) as PNG.
	RenderCaptcha(question string) ([]byte, error)
}
