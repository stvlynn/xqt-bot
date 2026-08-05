//go:build !js || !wasm

package main

import (
	"log"
	"net/http"
)

// The host entrypoint serves the webhook locally (e.g. behind a tunnel for
// development). The KV store is in-memory on this target.
func main() {
	handler, _, err := setup()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("xqt-bot listening on :8787 (POST /webhook)")
	log.Fatal(http.ListenAndServe(":8787", handler))
}
