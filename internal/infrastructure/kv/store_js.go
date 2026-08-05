//go:build js && wasm

package kv

import (
	"context"
	"fmt"
	"sort"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
	"github.com/syumai/workers/cloudflare/kv"
)

// cfStore implements Store against a Cloudflare KV namespace.
type cfStore struct {
	ns *kv.Namespace
}

// NewStore binds to the Cloudflare KV namespace configured under varName
// (the kv_namespaces binding in wrangler.toml).
func NewStore(varName string) (Store, error) {
	ns, err := kv.NewNamespace(varName)
	if err != nil {
		return nil, fmt.Errorf("kv: binding %q: %w", varName, err)
	}
	return &cfStore{ns: ns}, nil
}

// Get implements Store. Cloudflare KV resolves missing keys to null, which
// syumai/workers surfaces as the literal string "<null>"; our values are
// always JSON and never equal to that sentinel.
func (s *cfStore) Get(_ context.Context, key string) ([]byte, error) {
	val, err := s.ns.GetString(key, nil)
	if err != nil {
		return nil, fmt.Errorf("kv: get %q: %w", key, err)
	}
	if val == "<null>" {
		return nil, ports.ErrNotFound
	}
	return []byte(val), nil
}

// Put implements Store.
func (s *cfStore) Put(_ context.Context, key string, val []byte, ttlSeconds int) error {
	opts := &kv.PutOptions{}
	if ttlSeconds > 0 {
		opts.ExpirationTTL = ttlSeconds
	}
	if err := s.ns.PutString(key, string(val), opts); err != nil {
		return fmt.Errorf("kv: put %q: %w", key, err)
	}
	return nil
}

// Delete implements Store.
func (s *cfStore) Delete(_ context.Context, key string) error {
	if err := s.ns.Delete(key); err != nil {
		return fmt.Errorf("kv: delete %q: %w", key, err)
	}
	return nil
}

// ListKeys implements Store, following the KV list cursor to completion.
func (s *cfStore) ListKeys(_ context.Context, prefix string) ([]string, error) {
	keys := make([]string, 0)
	cursor := ""
	for {
		res, err := s.ns.List(&kv.ListOptions{Prefix: prefix, Cursor: cursor})
		if err != nil {
			return nil, fmt.Errorf("kv: list %q: %w", prefix, err)
		}
		for _, k := range res.Keys {
			keys = append(keys, k.Name)
		}
		if res.ListComplete || len(res.Keys) == 0 {
			break
		}
		cursor = res.Cursor
	}
	sort.Strings(keys)
	return keys, nil
}
