# xqt-bot（小群体）

一个跑在 **Cloudflare Workers** 上的 Telegram 群组管理 bot，纯 **Go** 编写（编译为 WebAssembly），MIT 协议开源。

把 bot 拉进群并设为管理员，它就能帮你：

- **拉人进群**：生成 `t.me` 拉人链接，群成员点开后私聊 bot，获得**仅一次、限时有效**的入群邀请链接
- **群管理**：内置广告/诈骗敏感词库 + 自定义关键词/正则，命中自动删消息并禁言；新成员进群人机验证（按钮或图片验证码）；回复消息一键 `/kick`、`/ban`、`/mute`
- **群聊总结**：接入任意 OpenAI 兼容大模型，`/summary` 一键总结最近 N 小时聊天，也可开启每 N 小时自动总结
- **定时任务**：按周期自动总结、自动清理长期不发言的僵尸成员
- **自动表情**：关键词/正则触发 emoji reaction，也可交给大模型自由发挥
- **娱乐**：`/roll` 掷点、`/pick` 帮你做选择、可配置的新成员欢迎语

## 快速开始（部署你自己的）

需要：Go 1.25+、Node.js、一个 Cloudflare 账号、一个 Telegram bot token（找 @BotFather 申请）。

```bash
git clone https://github.com/stvlynn/xqt-bot.git
cd xqt-bot
go mod download && npm install -g wrangler
wrangler login

make kv-setup        # 创建 KV 命名空间并写回 wrangler.toml
make secrets         # 交互式设置 bot token / webhook 密钥 / LLM key
make deploy          # 构建 wasm 并部署到 Workers
make webhook-setup   # 告诉 Telegram webhook 地址 + 注册命令菜单
```

然后把 bot 拉进群、设为管理员，群里发 `/xqt` 打开管理面板。

> Cloudflare 支持直接在 Dashboard 关联本 GitHub 仓库自动部署：Workers → 创建 → 导入仓库，构建命令 `npm run build`，入口 `build/worker.mjs`，并配置同名 KV binding 与 secrets 即可。

### 配置项

| 类型 | 名称 | 说明 |
| --- | --- | --- |
| secret | `TELEGRAM_BOT_TOKEN` | BotFather 颁发的 token |
| secret | `TELEGRAM_WEBHOOK_SECRET` | 随机字符串，校验 webhook 来源 |
| secret | `LLM_API_KEY` | OpenAI 兼容 API key（可选，不配置则 AI 功能关闭） |
| var | `LLM_BASE_URL` / `LLM_MODEL` | 模型端点与型号，默认 OpenAI `gpt-4o-mini`，可指向 Cloudflare AI Gateway |
| var | `BOT_USERNAME` | bot 用户名（不含 @），用于生成拉人链接 |

本地开发：`cp .dev.vars.example .dev.vars` 填入密钥后 `make dev`。

## 群管命令速查

| 命令 | 作用 |
| --- | --- |
| `/xqt` | 管理面板（开关验证、自动总结、查看配置） |
| `/invite` | 生成拉人链接，转发给谁谁能进 |
| `/filter add 词` `/filter add /正则/` `/filter del 词` | 管理敏感词 |
| `/captcha on\|off\|button\|image` | 进群验证 |
| `/kick` `/ban` `/mute 10` `/unmute` | 回复目标消息使用 |
| `/autoreact 词 表情`、`/autoreact /正则/ 表情`、`/autoreact del 词`、`/autoreact llm on` | 自动表情 |
| `/summary [小时]`、`/summary auto 6`、`/summary auto off` | 群聊总结 |
| `/clean`、`/clean go`、`/clean days 45` | 僵尸成员清理 |
| `/welcome 文本`、`/welcome off` | 欢迎语，支持 `{name}` `{chat}` |

## 技术栈与架构

- Go → WASM（[`syumai/workers`](https://github.com/syumai/workers)），webhook 模式（[`go-telegram/bot`](https://github.com/go-telegram/bot)）
- Cloudflare KV 存储配置/规则/消息日志/定时任务；Cron Triggers 每 5 分钟驱动到期任务
- 严格 DDD 分层：`internal/domain`（实体+端口）→ `internal/application`（用例）→ `internal/infrastructure`（KV/Telegram/LLM/图片）→ `internal/interfaces`（bot 命令/菜单/webhook/cron）
- 架构细节见 [`docs/project/architecture.md`](docs/project/architecture.md)，贡献者请先读 [`CLAUDE.md`](CLAUDE.md)

## 开发

```bash
make check   # vet + 单测 + wasm 构建
make test    # 单元测试
make fmt     # gofmt
```

## License

[MIT](LICENSE) © stvlynn
