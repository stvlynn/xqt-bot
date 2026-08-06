// Command xqt-bot is the Telegram group-management bot. It compiles to two
// targets: a Cloudflare Workers WebAssembly module (main_js.go) and a plain
// host binary for local debugging and tests (main_host.go).
package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/stvlynn/xqt-bot/internal/application"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/config"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/image"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/kv"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/llm"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/telegram"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/wordlist"
	"github.com/stvlynn/xqt-bot/internal/interfaces/bot"
	ifhttp "github.com/stvlynn/xqt-bot/internal/interfaces/http"
)

// setup wires the whole application: config, adapters, repositories,
// services, the update handler and the HTTP mux. It runs on both targets;
// platform differences are isolated in newStore and the main entrypoints.
func setup() (http.Handler, *application.TaskRunner, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	// WithSkipGetMe: the wasm module must not issue network requests during
	// startup; the telegram gateway resolves the bot's own ID lazily.
	// The injected http client uses the Workers-safe fetch transport on
	// js/wasm and the standard transport on the host.
	httpClient := newHTTPClient()
	b, err := tgbot.New(cfg.TelegramToken,
		tgbot.WithSkipGetMe(),
		tgbot.WithHTTPClient(0, httpClient),
	)
	if err != nil {
		return nil, nil, err
	}
	tg := telegram.NewGateway(b, 0)

	store, err := newStore(cfg)
	if err != nil {
		return nil, nil, err
	}
	settingsRepo := kv.NewSettingsRepository(store)
	captchaRepo := kv.NewCaptchaRepository(store)
	msglogRepo := kv.NewMessageLogRepository(store)
	activityRepo := kv.NewActivityRepository(store)
	taskRepo := kv.NewTaskRepository(store)

	// LLM options: temperature is only sent when explicitly configured;
	// endpoints like kimi-for-coding reject any explicit value.
	var llmOpts []llm.Option
	if cfg.LLMTemperature != "" {
		t, err := strconv.ParseFloat(cfg.LLMTemperature, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("config: invalid LLM_TEMPERATURE %q: %w", cfg.LLMTemperature, err)
		}
		llmOpts = append(llmOpts, llm.WithTemperature(t))
	}
	llmGateway := llm.NewGatewayWithClient(cfg.LLMBaseURL, cfg.LLMModel, cfg.LLMAPIKey, httpClient, llmOpts...)
	renderer, err := image.NewRenderer()
	if err != nil {
		return nil, nil, err
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	wordlistGateway := wordlist.NewGateway(httpClient)
	captchaSvc := application.NewCaptchaService(settingsRepo, captchaRepo, tg, renderer, rng)
	settingsSvc := application.NewSettingsService(settingsRepo, tg)
	moderationSvc := application.NewModerationService(settingsRepo, taskRepo, tg, wordlistGateway)
	inviteSvc := application.NewInviteService(settingsRepo, tg, cfg.BotUsername)
	reactionSvc := application.NewReactionService(settingsRepo, tg, llmGateway)
	summarySvc := application.NewSummaryService(settingsRepo, msglogRepo, taskRepo, tg, llmGateway)
	zombieSvc := application.NewZombieService(settingsRepo, activityRepo, tg)
	funSvc := application.NewFunService()
	pipeline := application.NewGroupMessagePipeline(moderationSvc, reactionSvc, summarySvc, zombieSvc)

	handler := bot.NewHandler(bot.Deps{
		Telegram:       tg,
		LLM:            llmGateway,
		Captcha:        captchaSvc,
		Settings:       settingsSvc,
		Moderation:     moderationSvc,
		Invite:         inviteSvc,
		Reaction:       reactionSvc,
		Summary:        summarySvc,
		Zombie:         zombieSvc,
		Fun:            funSvc,
		Pipeline:       pipeline,
		BotUsername:    cfg.BotUsername,
		DefaultListURL: cfg.FilterListURL,
		RNG:            rng,
	})
	runner := application.NewTaskRunner(taskRepo, summarySvc, zombieSvc, captchaSvc, moderationSvc, tg, settingsRepo)

	return ifhttp.NewMux(cfg, handler), runner, nil
}
