// Package kv implements the domain repositories on top of a simple
// key-value Store abstraction. The production Store is a Cloudflare KV
// namespace (store_js.go); tests and local runs use MemoryStore.
package kv

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/stvlynn/xqt-bot/internal/domain/ports"
)

// Store is the minimal key-value contract the repositories need.
// Get returns ports.ErrNotFound when the key does not exist.
type Store interface {
	// Get returns the raw value for key, or ports.ErrNotFound.
	Get(ctx context.Context, key string) ([]byte, error)
	// Put stores val under key; ttlSeconds <= 0 means no expiry.
	Put(ctx context.Context, key string, val []byte, ttlSeconds int) error
	// Delete removes key. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
	// ListKeys returns every key starting with prefix (sorted).
	ListKeys(ctx context.Context, prefix string) ([]string, error)
}

// MemoryStore is an in-memory Store for unit tests and local development.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string][]byte
}

// NewMemoryStore creates an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{entries: make(map[string][]byte)}
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.entries[key]
	if !ok {
		return nil, ports.ErrNotFound
	}
	out := make([]byte, len(val))
	copy(out, val)
	return out, nil
}

// Put implements Store. TTL is ignored: expiry sweeps in this project are
// driven by stored timestamps, not by storage-level expiry.
func (s *MemoryStore) Put(_ context.Context, key string, val []byte, _ int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(val))
	copy(cp, val)
	s.entries[key] = cp
	return nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

// ListKeys implements Store.
func (s *MemoryStore) ListKeys(_ context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	keys := make([]string, 0)
	for k := range s.entries {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}
