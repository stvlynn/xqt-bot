// Package config loads the bot's runtime configuration from the worker
// environment (Cloudflare bindings/secrets under js/wasm, process env on
// a host for local runs and tests).
package config

import (
	"fmt"
)

// envGet reads one environment value. It is implemented per-platform in
// env_js.go (Cloudflare runtime) and env_host.go (os.Getenv).
type envGetter func(key string) string

// Config is the resolved bot configuration.
type Config struct {
	// TelegramToken is the Bot API token (env TELEGRAM_BOT_TOKEN, required).
	TelegramToken string
	// WebhookSecret authenticates incoming webhook requests
	// (env TELEGRAM_WEBHOOK_SECRET).
	WebhookSecret string
	// BotUsername is the bot's @username used for deep links (env BOT_USERNAME).
	BotUsername string
	// KVBinding is the name of the Cloudflare KV namespace binding
	// (env KV_BINDING, default "KV").
	KVBinding string
	// LLMBaseURL is an OpenAI-compatible endpoint base (env LLM_BASE_URL).
	LLMBaseURL string
	// LLMModel is the chat-completions model name (env LLM_MODEL).
	LLMModel string
	// LLMAPIKey authorizes the LLM endpoint (env LLM_API_KEY). When empty,
	// the LLM gateway reports Available() == false.
	LLMAPIKey string
	// FilterListURL is the default remote word list imported by a bare
	// "/filter import" (env FILTER_LIST_URL, optional).
	FilterListURL string
	// Environment distinguishes dev/staging/prod (env ENVIRONMENT).
	Environment string
}

// DefaultKVBinding is used when no binding name is configured.
const DefaultKVBinding = "KV"

// Load reads configuration from the environment. Only TelegramToken is
// mandatory; everything else degrades gracefully when absent.
func Load() (*Config, error) {
	return load(envGet)
}

// load is the platform-independent core, kept separate so it can be tested
// on the host with a fake getter.
func load(get envGetter) (*Config, error) {
	cfg := &Config{
		TelegramToken: get("TELEGRAM_BOT_TOKEN"),
		WebhookSecret: get("TELEGRAM_WEBHOOK_SECRET"),
		BotUsername:   get("BOT_USERNAME"),
		KVBinding:     get("KV_BINDING"),
		LLMBaseURL:    get("LLM_BASE_URL"),
		LLMModel:      get("LLM_MODEL"),
		LLMAPIKey:     get("LLM_API_KEY"),
		FilterListURL: get("FILTER_LIST_URL"),
		Environment:   get("ENVIRONMENT"),
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("config: TELEGRAM_BOT_TOKEN is required")
	}
	if cfg.KVBinding == "" {
		cfg.KVBinding = DefaultKVBinding
	}
	return cfg, nil
}
