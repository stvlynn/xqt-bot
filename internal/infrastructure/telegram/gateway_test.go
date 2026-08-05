package telegram

import (
	"testing"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

func TestBuildMarkupNilForNoButtons(t *testing.T) {
	if got := buildMarkup(nil); got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
	if got := buildMarkup([][]ports.Button{{}}); got != nil {
		t.Errorf("expected nil for empty rows, got %+v", got)
	}
}

func TestBuildMarkupMapsURLAndCallbackData(t *testing.T) {
	markup := buildMarkup([][]ports.Button{
		{
			{Text: "open", URL: "https://example.com"},
			{Text: "answer 7", Data: "captcha:7"},
		},
		{
			{Text: "answer 5", Data: "captcha:5"},
		},
	})
	if markup == nil {
		t.Fatal("expected markup")
	}
	if len(markup.InlineKeyboard) != 2 {
		t.Fatalf("rows = %d, want 2", len(markup.InlineKeyboard))
	}
	row0 := markup.InlineKeyboard[0]
	if len(row0) != 2 {
		t.Fatalf("row 0 buttons = %d, want 2", len(row0))
	}
	if row0[0].Text != "open" || row0[0].URL != "https://example.com" || row0[0].CallbackData != "" {
		t.Errorf("URL button mapped wrong: %+v", row0[0])
	}
	if row0[1].CallbackData != "captcha:7" || row0[1].URL != "" {
		t.Errorf("callback button mapped wrong: %+v", row0[1])
	}
	if markup.InlineKeyboard[1][0].CallbackData != "captcha:5" {
		t.Errorf("second row mapped wrong: %+v", markup.InlineKeyboard[1][0])
	}
}
