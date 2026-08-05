//go:build js && wasm

package main

import (
	"github.com/stvlynn/xqt-bot/internal/infrastructure/config"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/kv"
)

// newStore binds to the Cloudflare KV namespace configured in wrangler.toml.
func newStore(cfg *config.Config) (kv.Store, error) {
	return kv.NewStore(cfg.KVBinding)
}
