package functions

import (
	"sync"
	"time"
)

type kvEntry struct {
	value     interface{}
	expiresAt time.Time
	hasTTL    bool
}

// ProjectKV is an in-memory key-value store scoped to one project.
type ProjectKV struct {
	mu      sync.RWMutex
	entries map[string]kvEntry
}

func newProjectKV() *ProjectKV {
	return &ProjectKV{entries: make(map[string]kvEntry)}
}

func (kv *ProjectKV) Set(key string, value interface{}, ttlSeconds int) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	e := kvEntry{value: value}
	if ttlSeconds > 0 {
		e.hasTTL = true
		e.expiresAt = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	}
	kv.entries[key] = e
}

func (kv *ProjectKV) Get(key string) (interface{}, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	e, ok := kv.entries[key]
	if !ok {
		return nil, false
	}
	if e.hasTTL && time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.value, true
}

func (kv *ProjectKV) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.entries, key)
}

func (kv *ProjectKV) Evict() int {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	now := time.Now()
	removed := 0
	for k, e := range kv.entries {
		if e.hasTTL && now.After(e.expiresAt) {
			delete(kv.entries, k)
			removed++
		}
	}
	return removed
}

// kvStore holds one ProjectKV per project ID.
var (
	kvMu    sync.RWMutex
	kvStore = map[string]*ProjectKV{}
)

// GetProjectKV returns (creating if needed) the KV store for a project.
func GetProjectKV(projectID string) *ProjectKV {
	kvMu.RLock()
	kv, ok := kvStore[projectID]
	kvMu.RUnlock()
	if ok {
		return kv
	}
	kvMu.Lock()
	defer kvMu.Unlock()
	if kv, ok = kvStore[projectID]; ok {
		return kv
	}
	kv = newProjectKV()
	kvStore[projectID] = kv
	return kv
}

// EvictAllExpired removes expired entries from every project's KV store.
func EvictAllExpired() int {
	kvMu.RLock()
	stores := make([]*ProjectKV, 0, len(kvStore))
	for _, kv := range kvStore {
		stores = append(stores, kv)
	}
	kvMu.RUnlock()

	total := 0
	for _, kv := range stores {
		total += kv.Evict()
	}
	return total
}
