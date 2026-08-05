//go:build !js || !wasm

package config

import (
	"os"
)

// envGet reads variables from the process environment (local dev, tests).
func envGet(key string) string {
	return os.Getenv(key)
}
