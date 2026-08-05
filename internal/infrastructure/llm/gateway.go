// Package llm implements ports.LLMGateway against any OpenAI-compatible
// chat-completions endpoint. It is plain net/http and compiles for both
// host (tests) and js/wasm (the worker).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/stvlynn/xqt-bot/internal/domain/summary"
)

// Tuning constants for the chat-completions request.
const (
	temperature        = 0.3
	maxTokens          = 800
	maxInputRunes      = 12000
	maxMessageRunes    = 200
	noReaction         = "NONE"
	summarizePrompt    = "你是群聊总结助手。请用中文总结下面的群聊记录：分要点列出，突出讨论的话题与待办事项，全文不超过 300 字。只输出总结本身，不要任何额外说明。"
	pickReactPromptT   = "你是一个表情回应助手。给定一条群聊消息，如果它值得一个表情回应，就从白名单 %s 中只输出一个 emoji；如果不值得回应，只输出 NONE。不要输出任何其他内容。"
	errNotConfigured   = "llm: not configured (LLM_API_KEY is empty)"
	defaultHTTPTimeout = 30 * time.Second
)

// Gateway implements ports.LLMGateway.
type Gateway struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// NewGateway creates the client. An empty apiKey yields a gateway with
// Available() == false whose calls fail fast with a clear error.
func NewGateway(baseURL, model, apiKey string) *Gateway {
	return NewGatewayWithClient(baseURL, model, apiKey, &http.Client{
		Timeout: defaultHTTPTimeout,
	})
}

// NewGatewayWithClient creates the client with a caller-provided http.Client.
// The worker entrypoint uses this to inject the Workers-safe fetch transport.
func NewGatewayWithClient(baseURL, model, apiKey string, client *http.Client) *Gateway {
	return &Gateway{
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		apiKey:     apiKey,
		httpClient: client,
	}
}

var _ ports.LLMGateway = (*Gateway)(nil)

// Available implements ports.LLMGateway.
func (g *Gateway) Available() bool {
	return g.apiKey != ""
}

// chatMessage is one message in the OpenAI chat-completions schema.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest is the OpenAI chat-completions request body.
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// chatResponse is the subset of the completion response we read.
type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Summarize implements ports.LLMGateway.
func (g *Gateway) Summarize(ctx context.Context, messages []summary.Message) (string, error) {
	if !g.Available() {
		return "", fmt.Errorf(errNotConfigured)
	}
	return g.complete(ctx, []chatMessage{
		{Role: "system", Content: summarizePrompt},
		{Role: "user", Content: formatMessages(messages)},
	})
}

// PickReaction implements ports.LLMGateway. The reply is accepted only when
// it exactly matches a whitelist entry or NONE; anything else yields
// ok=false without an error.
func (g *Gateway) PickReaction(ctx context.Context, text string, whitelist []string) (string, bool, error) {
	if !g.Available() {
		return "", false, fmt.Errorf(errNotConfigured)
	}
	list, err := json.Marshal(whitelist)
	if err != nil {
		return "", false, fmt.Errorf("llm: encode whitelist: %w", err)
	}
	reply, err := g.complete(ctx, []chatMessage{
		{Role: "system", Content: fmt.Sprintf(pickReactPromptT, string(list))},
		{Role: "user", Content: text},
	})
	if err != nil {
		return "", false, err
	}
	reply = strings.TrimSpace(reply)
	if reply == noReaction || reply == "" {
		return "", false, nil
	}
	for _, emoji := range whitelist {
		if reply == emoji {
			return emoji, true, nil
		}
	}
	return "", false, nil
}

// complete performs one chat-completions round trip.
func (g *Gateway) complete(ctx context.Context, messages []chatMessage) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       g.model,
		Messages:    messages,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("llm: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("llm: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: request: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: request failed with status %d: %s", resp.StatusCode, truncate(string(payload), 200))
	}
	var decoded chatResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", fmt.Errorf("llm: decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("llm: response has no choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

// formatMessages renders the message log as `HH:MM name: text` lines,
// truncating each message and the whole block to keep the prompt bounded.
func formatMessages(messages []summary.Message) string {
	var b strings.Builder
	for _, m := range messages {
		fmt.Fprintf(&b, "%02d:%02d %s: %s\n",
			m.At.Hour(), m.At.Minute(), m.UserName, truncate(m.Text, maxMessageRunes))
	}
	return truncate(b.String(), maxInputRunes)
}

// truncate cuts s to at most n runes.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n])
}
