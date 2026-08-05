package wordlist

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stvlynn/xqt-bot/internal/domain/moderation"
)

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchSuccess(t *testing.T) {
	srv := serve(t, http.StatusOK, "# comment\n刷单\n/(?i)usdt/\n\n")
	g := NewGateway(srv.Client())

	rules, err := g.Fetch(context.Background(), srv.URL+"/list.txt")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %+v", rules)
	}
	if rules[0].Kind != moderation.RuleWord || rules[0].Pattern != "刷单" {
		t.Fatalf("unexpected word rule: %+v", rules[0])
	}
	if rules[1].Kind != moderation.RuleRegex || rules[1].Pattern != "(?i)usdt" {
		t.Fatalf("unexpected regex rule: %+v", rules[1])
	}
	wantSource := srv.URL + "/list.txt"
	for _, r := range rules {
		if r.Source != wantSource {
			t.Fatalf("want source %q, got %+v", wantSource, r)
		}
	}
}

func TestFetchNon200(t *testing.T) {
	srv := serve(t, http.StatusNotFound, "nope")
	g := NewGateway(srv.Client())

	_, err := g.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("want status error mentioning 404, got %v", err)
	}
}

func TestFetchOversizedBody(t *testing.T) {
	srv := serve(t, http.StatusOK, strings.Repeat("a", maxBodyBytes+1))
	g := NewGateway(srv.Client())

	_, err := g.Fetch(context.Background(), srv.URL)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("want size-limit error, got %v", err)
	}
}

func TestFetchInvalidScheme(t *testing.T) {
	g := NewGateway(http.DefaultClient)
	for _, url := range []string{"ftp://example.com/list.txt", "file:///etc/passwd"} {
		if _, err := g.Fetch(context.Background(), url); err == nil ||
			!strings.Contains(err.Error(), "scheme") {
			t.Fatalf("url %q: want scheme error, got %v", url, err)
		}
	}
}
