#!/usr/bin/env bash
# Sets the worker secrets interactively. Values never touch the repo.
set -euo pipefail

put_secret() {
  local name="$1" prompt="$2"
  local value
  read -r -s -p "$prompt: " value
  echo
  if [[ -z "$value" ]]; then
    echo "Skipping $name (empty)."
    return
  fi
  printf '%s' "$value" | wrangler secret put "$name"
  echo "Set $name."
}

put_secret TELEGRAM_BOT_TOKEN "Telegram bot token (from @BotFather)"
put_secret TELEGRAM_WEBHOOK_SECRET "Webhook secret token (any random string)"
put_secret LLM_API_KEY "LLM API key (OpenAI-compatible; optional, press Enter to skip)"
