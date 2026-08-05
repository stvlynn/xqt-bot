// Package wordlist implements ports.WordListGateway: it downloads a remote
// sensitive-word list over HTTP and parses it with the domain parser. It is
// plain net/http and compiles for both host (tests) and js/wasm (the worker).
package wordlist

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

const (
	// maxBodyBytes caps one word-list download at 1 MiB.
	maxBodyBytes = 1 << 20
	// fetchTimeout bounds a single list download.
	fetchTimeout = 10 * time.Second
)

// Gateway implements ports.WordListGateway.
type Gateway struct {
	client *http.Client
}

// NewGateway creates the gateway with a caller-provided http.Client. The
// worker entrypoint uses this to inject the Workers-safe fetch transport.
func NewGateway(client *http.Client) *Gateway {
	return &Gateway{client: client}
}

var _ ports.WordListGateway = (*Gateway)(nil)

// Fetch implements ports.WordListGateway.
func (g *Gateway) Fetch(ctx context.Context, url string) ([]moderation.FilterRule, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("wordlist: build request: %w", err)
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("wordlist: unsupported URL scheme %q", req.URL.Scheme)
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wordlist: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wordlist: request failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("wordlist: read response: %w", err)
	}
	if len(body) > maxBodyBytes {
		return nil, fmt.Errorf("wordlist: response exceeds %d bytes", maxBodyBytes)
	}
	rules, _ := moderation.ParseWordList(string(body), url)
	return rules, nil
}
