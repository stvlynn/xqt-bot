#!/usr/bin/env bash
# Registers the Telegram webhook and command menu for the deployed worker.
#
# Required env:
#   TELEGRAM_BOT_TOKEN      bot token from @BotFather
#   TELEGRAM_WEBHOOK_SECRET same value as the worker secret
#   WORKER_URL              e.g. https://xqt-bot.<subdomain>.workers.dev
set -euo pipefail

: "${TELEGRAM_BOT_TOKEN:?set TELEGRAM_BOT_TOKEN}"
: "${TELEGRAM_WEBHOOK_SECRET:?set TELEGRAM_WEBHOOK_SECRET}"
: "${WORKER_URL:?set WORKER_URL, e.g. https://xqt-bot.example.workers.dev}"

API="https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}"

echo "Setting webhook -> ${WORKER_URL}/webhook"
curl -fsS -X POST "${API}/setWebhook" \
  -H 'Content-Type: application/json' \
  -d "{
    \"url\": \"${WORKER_URL}/webhook\",
    \"secret_token\": \"${TELEGRAM_WEBHOOK_SECRET}\",
    \"allowed_updates\": [\"message\", \"callback_query\", \"my_chat_member\"],
    \"drop_pending_updates\": true
  }"
echo

echo "Registering command menu"
curl -fsS -X POST "${API}/setMyCommands" \
  -H 'Content-Type: application/json' \
  -d '{
    "commands": [
      {"command": "start",     "description": "开始使用 / 通过邀请链接进群"},
      {"command": "help",      "description": "查看使用说明"},
      {"command": "xqt",       "description": "打开群管理面板"},
      {"command": "invite",    "description": "生成拉人链接（一次性、限时）"},
      {"command": "filter",    "description": "敏感词管理：/filter add|del [词或/正则/]"},
      {"command": "captcha",   "description": "进群验证：/captcha on|off|button|image"},
      {"command": "kick",      "description": "回复消息使用：踢出该成员"},
      {"command": "ban",       "description": "回复消息使用：封禁该成员"},
      {"command": "mute",      "description": "回复消息使用：禁言，/mute 10 表示 10 分钟"},
      {"command": "unmute",    "description": "回复消息使用：解除禁言"},
      {"command": "autoreact", "description": "自动表情：/autoreact 关键词 表情"},
      {"command": "summary",   "description": "生成群聊总结：/summary [小时数]"},
      {"command": "clean",     "description": "清理僵尸成员：/clean 预览，/clean go 执行"},
      {"command": "welcome",   "description": "设置欢迎语：/welcome 文本，/welcome off 关闭"},
      {"command": "roll",      "description": "掷一个 1-100 的随机数"},
      {"command": "pick",      "description": "帮你选择：/pick 选项A 选项B ..."}
    ]
  }'
echo

echo "Current webhook info:"
curl -fsS "${API}/getWebhookInfo"
echo
