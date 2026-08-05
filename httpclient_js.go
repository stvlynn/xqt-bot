//go:build js && wasm

package main

import (
	"net/http"

	"github.com/syumai/workers/cloudflare/fetch"
)

// newHTTPClient returns a client whose transport calls the Workers fetch API
// with the correct receiver. Go's default js/wasm transport triggers
// "Illegal invocation" on the Workers runtime.
func newHTTPClient() *http.Client {
	return fetch.NewClient().HTTPClient(fetch.RedirectModeFollow)
}
