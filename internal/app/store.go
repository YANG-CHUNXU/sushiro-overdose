package app

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type StoreEntry struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

type StoreRegistry struct {
	mu      sync.RWMutex
	entries map[string]StoreEntry
	path    string
}

var (
	globalRegistry     *StoreRegistry
	globalRegistryOnce sync.Once
)

func storeRegistryPath() string {
	return fmt.Sprintf("%s/stores.json", appDirPath())
}

func loadStoreRegistry() *StoreRegistry {
	r := &StoreRegistry{
		entries: map[string]StoreEntry{},
		path:    storeRegistryPath(),
	}
	data, err := os.ReadFile(r.path)
	if err == nil {
		var entries []StoreEntry
		if json.Unmarshal(data, &entries) == nil {
			for _, e := range entries {
				r.entries[e.ID] = e
			}
		}
	}
	return r
}

func GetStoreRegistry() *StoreRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = loadStoreRegistry()
	})
	return globalRegistry
}

func (r *StoreRegistry) DisplayName(storeID, fallback string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.entries[storeID]; ok && e.Nickname != "" {
		return e.Nickname
	}
	return fallback
}

func (r *StoreRegistry) Add(id, nickname string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = StoreEntry{ID: id, Nickname: nickname}
	r.save()
}

func (r *StoreRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
	r.save()
}

func (r *StoreRegistry) List() []StoreEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]StoreEntry, 0, len(r.entries))
	for _, e := range r.entries {
		result = append(result, e)
	}
	return result
}

func (r *StoreRegistry) save() {
	entries := make([]StoreEntry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.MkdirAll(appDirPath(), 0o755)
	_ = os.WriteFile(r.path, data, 0o600)
}
