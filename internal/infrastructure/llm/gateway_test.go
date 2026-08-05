package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// newServer runs a fake OpenAI-compatible endpoint returning content.
func newServer(t *testing.T, status int, content string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method %q", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing bearer auth")
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if req.Temperature != temperature || req.MaxTokens != maxTokens {
			t.Errorf("request params wrong: %+v", req)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			resp := chatResponse{}
			resp.Choices = append(resp.Choices, struct {
				Message chatMessage `json:"message"`
			}{Message: chatMessage{Role: "assistant", Content: content}})
			_ = json.NewEncoder(w).Encode(resp)
		} else {
			_, _ = w.Write([]byte(`{"error":"boom"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAvailable(t *testing.T) {
	if NewGateway("http://x", "m", "").Available() {
		t.Error("empty apiKey must report unavailable")
	}
	if !NewGateway("http://x", "m", "k").Available() {
		t.Error("configured apiKey must report available")
	}
}

func TestNotConfiguredFailsFast(t *testing.T) {
	g := NewGateway("http://unused", "m", "")
	if _, err := g.Summarize(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("Summarize err = %v", err)
	}
	if _, _, err := g.PickReaction(context.Background(), "hi", []string{"👍"}); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Errorf("PickReaction err = %v", err)
	}
}

func TestSummarizeSuccess(t *testing.T) {
	srv := newServer(t, http.StatusOK, "· 话题：发布计划\n· 待办：周五前完成")
	g := NewGateway(srv.URL, "model", "key")
	msgs := []summary.Message{
		{MessageID: 1, UserID: 1, UserName: "ann", Text: "hello", At: time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)},
	}
	got, err := g.Summarize(context.Background(), msgs)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !strings.Contains(got, "发布计划") {
		t.Errorf("Summarize = %q", got)
	}
}

func TestSummarizeNon200(t *testing.T) {
	srv := newServer(t, http.StatusInternalServerError, "")
	g := NewGateway(srv.URL, "model", "key")
	_, err := g.Summarize(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("err = %v, want status code in error", err)
	}
}

func TestPickReactionWhitelistHit(t *testing.T) {
	srv := newServer(t, http.StatusOK, "👍")
	g := NewGateway(srv.URL, "model", "key")
	emoji, ok, err := g.PickReaction(context.Background(), "great news!", []string{"👍", "🎉", "👀"})
	if err != nil {
		t.Fatalf("PickReaction: %v", err)
	}
	if !ok || emoji != "👍" {
		t.Errorf("emoji = %q ok = %v", emoji, ok)
	}
}

func TestPickReactionNone(t *testing.T) {
	srv := newServer(t, http.StatusOK, "NONE")
	g := NewGateway(srv.URL, "model", "key")
	_, ok, err := g.PickReaction(context.Background(), "ok", []string{"👍"})
	if err != nil {
		t.Fatalf("PickReaction: %v", err)
	}
	if ok {
		t.Error("NONE must yield ok=false")
	}
}

func TestPickReactionOutsideWhitelistRejected(t *testing.T) {
	srv := newServer(t, http.StatusOK, "🔥")
	g := NewGateway(srv.URL, "model", "key")
	_, ok, err := g.PickReaction(context.Background(), "msg", []string{"👍", "🎉"})
	if err != nil {
		t.Fatalf("PickReaction: %v", err)
	}
	if ok {
		t.Error("emoji outside whitelist must yield ok=false")
	}
}

func TestFormatMessagesTruncation(t *testing.T) {
	long := strings.Repeat("x", 500)
	msgs := []summary.Message{
		{UserName: "ann", Text: long, At: time.Date(2026, 1, 1, 9, 5, 0, 0, time.UTC)},
	}
	out := formatMessages(msgs)
	if !strings.HasPrefix(out, "09:05 ann: ") {
		t.Errorf("format wrong: %.30q", out)
	}
	// Single message truncated to maxMessageRunes plus prefix and newline.
	if n := len([]rune(out)); n > maxMessageRunes+20 {
		t.Errorf("message not truncated: %d runes", n)
	}
	// Whole block capped at maxInputRunes.
	var many []summary.Message
	for i := 0; i < 200; i++ {
		many = append(many, summary.Message{UserName: "u", Text: long})
	}
	if n := len([]rune(formatMessages(many))); n > maxInputRunes {
		t.Errorf("block not capped: %d runes", n)
	}
}
