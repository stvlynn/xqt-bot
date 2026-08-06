package bot

import (
	"fmt"

	"github.com/stvlynn/xqt-bot/internal/application"
)

// All user-facing copy lives here (Simplified Chinese, per project
// convention). Templates marked T are filled with fmt.Sprintf.

// Generic replies and error mapping (application sentinel errors).
const (
	textErrNotAdmin         = "🚫 只有群管理员可以这样做"
	textErrTargetIsAdmin    = "🚫 不能对管理员执行这个操作"
	textErrLLMNotConfigured = "未配置 AI 服务，这个功能暂时不可用"
	textErrTooFewMessages   = "最近的消息太少，还不足以生成总结"
	textErrDuplicate        = "这条规则已经存在了"
	textErrNotFound         = "没有找到对应的内容"
	textErrInvalidArgument  = "参数不对，请参考用法示例"
	textErrUnknown          = "😥 出了点问题，请稍后再试"

	textGroupOnly = "这个命令只能在群里使用"
	textOn        = "✅ 开启"
	textOff       = "❌ 关闭"
)

// /start and /help.
const (
	textStartIntro = "👋 我是群管 bot，帮你打理群聊：新成员验证、敏感词过滤、自动总结、僵尸成员清理……\n\n" +
		"把我拉进群并设为管理员，然后在群里发 /xqt 打开管理面板。\n发 /help 查看全部命令。"

	textStartPayloadInvalid = "这个链接无效或已经被用过了，请让群管理员在群里发 /invite 重新生成一个。"

	textHelp = `📖 命令一览

入群与邀请
/xqt — 管理面板：一键开关验证、过滤、AI 表情、自动总结
/invite — 生成拉人链接（一次性、限时）
/captcha on|off — 开关进群验证；/captcha button|image 切换验证方式
/welcome 文本 — 设置欢迎语，{name}、{chat} 会自动替换；/welcome off 关闭

内容管理
/filter — 查看敏感词规则；/filter add 词语 或 /filter add /正则/ 添加；/filter del 词语或编号 删除；/filter import 网址 导入远程词库；/filter update 刷新词库
/channel @频道 — 绑定频道，新消息自动转发到本群并带评论按钮；/channel off 解绑
/kick — 回复某人的消息：移出群聊
/ban — 回复某人的消息：封禁
/mute 分钟 — 回复某人的消息：禁言（默认 10 分钟）
/unmute — 回复某人的消息：解除禁言
/pin — 回复某条消息：置顶；/unpin 取消置顶
/clean — 预览僵尸成员；/clean go 执行清理；/clean days 天数 设置多久不发言算僵尸

AI 与娱乐
/autoreact — 查看表情规则；/autoreact 词语 表情 或 /autoreact /正则/ 表情 添加；/autoreact del 词语 删除；/autoreact llm on|off 开关 AI 表情
/summary 小时数 — 总结最近聊天（默认 24 小时）；/summary auto 小时数 开启定时总结，/summary auto off 关闭
/roll — 掷一个 1-100 的随机数
/pick A B C — 帮你随机选一个`
)

// Invite flow.
const (
	// inviteReadyT is the private-chat reply after a deep link resolves:
	// chat title, validity minutes.
	inviteReadyT = "点击下方按钮加入「%s」：\n链接只能用一次，%d 分钟内有效。"
	// inviteReadyNoTitleT is inviteReadyT for chats whose title could not be
	// resolved: validity minutes.
	inviteReadyNoTitleT = "点击下方按钮加入群聊：\n链接只能用一次，%d 分钟内有效。"
	// inviteJoinButtonT is the URL-button label: chat title.
	inviteJoinButtonT = "加入「%s」"
	// inviteJoinButtonNoTitle is the button label without a resolved title.
	inviteJoinButtonNoTitle = "加入群聊"
	// inviteShareT is the group reply to /invite: share link, validity minutes.
	inviteShareT = "🔗 邀请链接已生成：\n%s\n\n转发给想邀请的人，对方点开会私聊我领取一次性入群链接（%d 分钟内有效）。"
)

// Join captcha.
const (
	// captchaChallengeT is the button-mode challenge message: new member
	// display name, arithmetic question.
	captchaChallengeT = "👋 欢迎 %s！\n请点选正确答案完成验证，超时将被移出群聊。\n\n🧮 %s"
	// captchaPhotoCaptionT is the image-mode photo caption: display name.
	captchaPhotoCaptionT = "👋 欢迎 %s！看上面的图算一算，点选正确答案完成验证，超时将被移出群聊。"
	textCaptchaWrong     = "答案不对，再试一次～"
	textCaptchaExpired   = "验证已超时，你已被移出群聊"
	textCaptchaPassed    = "✅ 验证通过，欢迎！"
	textCaptchaNotYours  = "这份验证是给新成员的，不用你点哦"
	textCaptchaGone      = "验证不存在或已经完成"
)

// Admin panel (/xqt).
const (
	// panelTextT: chat title, captcha state, filter state, filter rule count,
	// auto-react rule count, LLM reaction state, auto-summary state, welcome
	// state, zombie threshold days, invite validity minutes.
	panelTextT = `⚙️ 「%s」管理面板

进群验证：%s
敏感词过滤：%s（%d 条规则）
表情回应：%d 条规则；AI 表情：%s
自动总结：%s
欢迎语：%s
僵尸清理：%d 天未发言
邀请链接有效期：%d 分钟

点下方按钮直接开关：`
	// panelCaptchaOnT: captcha mode label (按钮/图片), timeout seconds.
	panelCaptchaOnT = "开启（%s验证，%d 秒）"
	// panelFilterOnT: hit action description.
	panelFilterOnT = "开启（%s）"
	// panelSummaryOnT: interval hours.
	panelSummaryOnT = "开启（每 %d 小时）"
	// panelButtonT: state emoji, feature label.
	panelButtonT     = "%s %s"
	textPanelRefresh = "🔄 刷新"
	textPanelUpdated = "已更新"
)

// /captcha.
const (
	textUsageCaptcha = "用法：\n/captcha on 开启进群验证\n/captcha off 关闭\n/captcha button 点按钮验证\n/captcha image 看图算数验证"
	// textCaptchaSetT: on/off state, mode label.
	textCaptchaSetT = "✅ 进群验证已%s（当前方式：%s）"
	// textCaptchaModeSetT: mode label.
	textCaptchaModeSetT   = "✅ 验证方式已切换为：%s"
	textCaptchaModeButton = "点按钮"
	textCaptchaModeImage  = "看图算数"
)

// /filter.
const (
	textUsageFilter = "用法：\n/filter 查看当前规则\n/filter add 词语 — 添加敏感词\n/filter add /正则/ — 添加正则规则\n/filter del 词语或编号 — 删除规则\n/filter import 网址 — 导入远程词库，如 /filter import https://example.com/list.txt（不带网址则用默认词库）\n/filter update — 刷新所有已导入的词库"
	// filterListHeaderT: on/off state, hit action.
	filterListHeaderT = "敏感词过滤：%s\n命中后：%s\n\n规则列表（/filter del 编号 可删除）："
	// filterRuleLineWordT / filterRuleLineRegexT: index, pattern.
	filterRuleLineWordT  = "%d. 词语：%s"
	filterRuleLineRegexT = "%d. 正则：/%s/"
	filterActionDeleteT  = "删除消息"
	filterActionMuteT    = "删除消息并禁言 %d 分钟"
	textFilterAddedT     = "✅ 已添加规则：%s"
	textFilterDeletedT   = "✅ 已删除规则：%s"
	textFilterNoRules    = "（暂无规则）"
	// filterSourcesLineT: imported source count.
	filterSourcesLineT = "已导入词库来源：%d 个（/filter import 导入，/filter update 刷新）"
	// textFilterImportedT: source URL, added count, skipped duplicates, total.
	textFilterImportedT = "✅ 词库导入完成\n来源：%s\n新增 %d 条，跳过重复 %d 条，当前共 %d 条规则"
	// textFilterRefreshedT: source count, net rule change.
	textFilterRefreshedT = "✅ 词库已刷新：%d 个来源，规则净变化 %+d 条"
	// textFilterRefreshFailedT: newline-joined failed source URLs.
	textFilterRefreshFailedT = "\n⚠️ 以下来源刷新失败：\n%s"
	textFilterNoSources      = "还没有导入过词库，先发 /filter import 网址 导入一个吧"
)

// Reply-target moderation commands.
const (
	textUsageReply  = "请回复要处理的成员的消息，再使用这个命令"
	textUsageMute   = "用法：回复某人的消息后发 /mute 分钟数（默认 10 分钟）"
	textUsagePin    = "用法：回复要置顶的消息后发 /pin；回复已置顶的消息发 /unpin 可取消置顶"
	textPinDone     = "📌 已置顶"
	textUnpinDone   = "已取消置顶"
	textKickDoneT   = "🦶 已将 %s 移出群聊"
	textBanDoneT    = "🔨 已封禁 %s"
	textMuteDoneT   = "🔇 已禁言 %s（%d 分钟）"
	textUnmuteDoneT = "🔊 已解除 %s 的禁言"
)

// /autoreact.
const (
	textUsageAutoReact = "用法：\n/autoreact 查看当前规则\n/autoreact 词语 表情 — 如 /autoreact 早上好 ☀️\n/autoreact /正则/ 表情 — 正则触发\n/autoreact del 词语 — 删除规则\n/autoreact llm on|off — 开关 AI 自动表情"
	// autoReactListHeaderT: rule count, LLM state.
	autoReactListHeaderT = "表情回应规则（共 %d 条），AI 表情：%s"
	// autoReactRuleLineKeywordT / autoReactRuleLineRegexT: index, pattern, emoji.
	autoReactRuleLineKeywordT = "%d. 词语：%s → %s"
	autoReactRuleLineRegexT   = "%d. 正则：/%s/ → %s"
	textAutoReactAddedT       = "✅ 已添加表情规则：%s → %s"
	textAutoReactDeletedT     = "✅ 已删除表情规则：%s"
	textAutoReactLLMSetT      = "✅ AI 表情已%s"
	textAutoReactNoRules      = "（暂无规则）"
)

// /summary.
const (
	textUsageSummary      = "用法：\n/summary 小时数 — 总结最近聊天（默认 24 小时）\n/summary auto 小时数 — 开启定时总结\n/summary auto off — 关闭定时总结"
	textSummaryGenerating = "⏳ 正在生成总结，请稍候…"
	// summaryResultT: hours, message count, summary text.
	summaryResultT     = "📝 最近 %d 小时群聊总结（共 %d 条消息）：\n\n%s"
	textAutoSummaryOnT = "✅ 已开启自动总结，每 %d 小时一次"
	textAutoSummaryOff = "✅ 已关闭自动总结"
)

// /clean.
const (
	textUsageClean = "用法：\n/clean — 预览将清理的僵尸成员\n/clean go — 执行清理\n/clean days 天数 — 设置多少天不发言算僵尸（1-365）"
	// cleanPreviewT: threshold days, zombie count.
	cleanPreviewT = "🧹 僵尸成员预览（%d 天未发言）：共 %d 人\n确认清理请发 /clean go"
	// cleanDoneT: kicked count, skipped count.
	cleanDoneT    = "🧹 清理完成：移出 %d 人，跳过 %d 人（管理员或无法移除）"
	cleanDaysSetT = "✅ 已设置：%d 天未发言视为僵尸成员"
)

// /welcome.
const (
	textUsageWelcome = "用法：\n/welcome 文本 — 设置欢迎语，{name}、{chat} 会自动替换，如：\n/welcome 欢迎 {name} 加入 {chat}！\n/welcome on 开启 / /welcome off 关闭"
	// welcomeStatusT: on/off state, current text.
	welcomeStatusT  = "欢迎语：%s\n当前内容：\n%s"
	textWelcomeSetT = "✅ 欢迎语已设置并开启：\n%s"
	// textWelcomeToggledT: on/off state.
	textWelcomeToggledT = "✅ 欢迎语已%s"
)

// /channel.
const (
	textUsageChannel = "用法：\n/channel @频道用户名 — 绑定频道，如 /channel @durov\n/channel off — 解绑\n/channel — 查看当前绑定"
	// channelBoundT: channel display name.
	channelBoundT = "✅ 已绑定频道 %s\n频道有新消息时，我会自动转发到本群，并附上评论按钮。"
	// channelBoundNoPreviewsT: channel display name.
	channelBoundNoPreviewsT   = "✅ 已绑定频道 %s\n频道有新消息时，我会自动转发到本群，并附上「去评论区」按钮。\n\n💡 把我也拉进该频道的讨论群并设为管理员，按钮里还能显示最新评论摘要。"
	textChannelUnbound        = "✅ 已解绑频道，之后不再转发它的新消息"
	textChannelNotBound       = "本群还没有绑定频道"
	textChannelLinkedHere     = "这个频道的讨论群就是本群，Telegram 已经会自动转发消息并带评论入口，无需绑定"
	textChannelNotFound       = "找不到这个频道，请检查用户名是否正确，如 /channel @durov"
	textChannelNotAChannel    = "这不是一个频道哦，/channel 只能绑定频道"
	textErrBotNotChannelAdmin = "我还不是该频道的管理员，请先把我加为频道管理员再绑定"
	// channelStatusT: channel display name, comment-preview state.
	channelStatusT = "当前绑定频道：%s\n评论摘要按钮：%s\n\n解绑请发 /channel off"
	// commentsButtonT: recorded comment count.
	commentsButtonT    = "💬 去评论区（%d）"
	commentsButtonZero = "💬 去评论区"
	// textCommentAnonymous labels comment authors Telegram hides.
	textCommentAnonymous = "匿名"
)

// ChannelLabels exposes the channel-feature button labels to the
// application layer without moving user-facing copy out of this package.
func ChannelLabels() application.ChannelLabels {
	return application.ChannelLabels{
		CommentsButton: func(count int) string {
			if count > 0 {
				return fmt.Sprintf(commentsButtonT, count)
			}
			return commentsButtonZero
		},
		AnonymousAuthor: textCommentAnonymous,
	}
}

// /roll and /pick.
const (
	// rollResultT: player roll, bot roll, outcome text.
	rollResultT  = "🎲 你掷出 %d 点，我掷出 %d 点 —— %s"
	textRollWin  = "你赢了！"
	textRollLose = "我赢了～"
	textRollDraw = "平局！"
	// pickResultT: chosen option.
	pickResultT   = "👉 我选：%s"
	textUsagePick = "至少给我两个选项，如：/pick 火锅 烧烤 日料"
)
