package application

import "errors"

// Sentinel errors returned by application services. The interfaces layer
// maps them to user-facing messages; keep them stable.
var (
	// ErrInvalidPayload is returned when a /start deep-link payload is not a
	// valid join payload.
	ErrInvalidPayload = errors.New("invalid invite payload")
	// ErrNotAdmin is returned when the requesting user is not a chat admin.
	ErrNotAdmin = errors.New("requester is not a chat administrator")
	// ErrTargetIsAdmin is returned when a moderation action targets an admin.
	ErrTargetIsAdmin = errors.New("target user is a chat administrator")
	// ErrDuplicate is returned when an identical rule already exists.
	ErrDuplicate = errors.New("rule already exists")
	// ErrNotFound is returned when the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrTooFewMessages is returned when a summary lacks enough source messages.
	ErrTooFewMessages = errors.New("too few messages to summarize")
	// ErrLLMNotConfigured is returned when an LLM-dependent feature has no backend.
	ErrLLMNotConfigured = errors.New("llm is not configured")
	// ErrInvalidArgument is returned for out-of-range or empty parameters.
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrChannelLinkedHere is returned when the channel to bind already
	// forwards into this group natively (its discussion group is this chat).
	ErrChannelLinkedHere = errors.New("channel discussion group is this chat")
	// ErrChannelNotFound is returned when a channel reference cannot be resolved.
	ErrChannelNotFound = errors.New("channel not found")
	// ErrNotAChannel is returned when a /channel reference resolves to a
	// non-channel chat.
	ErrNotAChannel = errors.New("target chat is not a channel")
	// ErrBotNotChannelAdmin is returned when the bot is not an administrator
	// of the channel to bind.
	ErrBotNotChannelAdmin = errors.New("bot is not an administrator of the channel")
)
