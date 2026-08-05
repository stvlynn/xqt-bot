//go:build !js || !wasm

package main

import (
	"github.com/stvlynn/xqt-bot/internal/infrastructure/config"
	"github.com/stvlynn/xqt-bot/internal/infrastructure/kv"
)

// newStore uses an in-memory store for host runs (local debugging, tests).
func newStore(_ *config.Config) (kv.Store, error) {
	return kv.NewMemoryStore(), nil
}
