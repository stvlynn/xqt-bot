package config

import (
	"testing"
)

func TestLoadRequiresTelegramToken(t *testing.T) {
	if _, err := load(func(string) string { return "" }); err == nil {
		t.Fatal("expected error when TELEGRAM_BOT_TOKEN is empty")
	}
}

func TestLoadDefaultsAndMapping(t *testing.T) {
	vars := map[string]string{
		"TELEGRAM_BOT_TOKEN": "tok",
		"LLM_BASE_URL":       "https://llm.example.com/v1",
		"LLM_MODEL":          "model-x",
		"LLM_API_KEY":        "key",
		"ENVIRONMENT":        "prod",
	}
	cfg, err := load(func(key string) string { return vars[key] })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.TelegramToken != "tok" {
		t.Errorf("TelegramToken = %q", cfg.TelegramToken)
	}
	if cfg.KVBinding != DefaultKVBinding {
		t.Errorf("KVBinding = %q, want default %q", cfg.KVBinding, DefaultKVBinding)
	}
	if cfg.LLMBaseURL != "https://llm.example.com/v1" || cfg.LLMModel != "model-x" || cfg.LLMAPIKey != "key" {
		t.Errorf("LLM fields not mapped: %+v", cfg)
	}
	if cfg.Environment != "prod" {
		t.Errorf("Environment = %q", cfg.Environment)
	}
}

func TestLoadKVBindingOverride(t *testing.T) {
	vars := map[string]string{
		"TELEGRAM_BOT_TOKEN": "tok",
		"KV_BINDING":         "BOT_KV",
	}
	cfg, err := load(func(key string) string { return vars[key] })
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.KVBinding != "BOT_KV" {
		t.Errorf("KVBinding = %q, want BOT_KV", cfg.KVBinding)
	}
}
