//go:build js && wasm

package config

import (
	"github.com/syumai/workers/cloudflare"
)

// envGet resolves variables from the Cloudflare worker runtime context
// (covers both bindings and secrets defined in wrangler.toml).
func envGet(key string) string {
	return cloudflare.Getenv(key)
}
