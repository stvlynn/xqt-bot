// Package http exposes the bot as an HTTP service: the Telegram webhook
// endpoint plus trivial health/root routes. It compiles for both host
// (tests, local debugging) and js/wasm (the worker).
package http

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-telegram/bot/models"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/config"
	"github.com/stvlynn/xqt-bot/internal/interfaces/bot"
)

// secretHeader is Telegram's webhook authentication header (set via
// setWebhook's secret_token).
const secretHeader = "X-Telegram-Bot-Api-Secret-Token"

// NewMux builds the HTTP routes: POST /webhook for Telegram updates,
// GET /healthz for liveness, GET / for a one-line description.
func NewMux(cfg *config.Config, h *bot.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /webhook", func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare(
			[]byte(r.Header.Get(secretHeader)), []byte(cfg.WebhookSecret)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var u models.Update
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		// Processing errors are logged, never reported: a non-200 would make
		// Telegram retry the update and storm the worker.
		if err := h.HandleUpdate(r.Context(), &u); err != nil {
			log.Printf("webhook: update %d: %v", u.ID, err)
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("xqt-bot: Telegram group management bot (webhook at /webhook)"))
	})

	return mux
}
