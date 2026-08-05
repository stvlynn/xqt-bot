//go:build !js || !wasm

package main

import (
	"net/http"
	"time"
)

// newHTTPClient returns the standard client for local development and tests.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}
