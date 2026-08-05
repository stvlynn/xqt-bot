package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stvlynn/xqt-bot/internal/infrastructure/config"
	"github.com/stvlynn/xqt-bot/internal/interfaces/bot"
)

func testMux() *http.ServeMux {
	cfg := &config.Config{WebhookSecret: "s3cret"}
	// An empty-deps handler is fine here: the 200-path test posts an empty
	// update, which touches no dependency.
	return NewMux(cfg, bot.NewHandler(bot.Deps{}))
}

func TestWebhookRejectsWrongSecret(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong")
	rec := httptest.NewRecorder()
	testMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestWebhookAcceptsCorrectSecret(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"update_id":1}`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "s3cret")
	rec := httptest.NewRecorder()
	testMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestWebhookRejectsBadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`not json`))
	req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "s3cret")
	rec := httptest.NewRecorder()
	testMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	testMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Fatalf("healthz = (%d, %q), want (200, ok)", rec.Code, rec.Body.String())
	}
}

func TestRoot(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	testMux().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "xqt-bot") {
		t.Fatalf("root = (%d, %q)", rec.Code, rec.Body.String())
	}
}
