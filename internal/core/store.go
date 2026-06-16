package core

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// StoreEntry 是用户给门店起的别名记录：ID 为官方门店号，Nickname 为用户自定义昵称。
type StoreEntry struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
}

// StoreRegistry 是门店别名的内存注册表，单例（GetStoreRegistry）。
// 并发约定：mu 是读写锁，读方法（DisplayName/List）用 RLock，写方法（Add/Remove/save）用 Lock。
// 注册表全程不缓存到磁盘以外的状态，save 负责把每次变更落盘到 ~/.sushiro/stores.json。
type StoreRegistry struct {
	mu      sync.RWMutex
	entries map[string]StoreEntry
	path    string
}

var (
	globalRegistry     *StoreRegistry
	globalRegistryOnce sync.Once
)

// StoreRegistryPath 返回门店别名的存储路径（~/.sushiro/stores.json）。
func StoreRegistryPath() string {
	return fmt.Sprintf("%s/stores.json", AppDirPath())
}

// loadStoreRegistry 从磁盘读 stores.json 初始化注册表。文件缺失或解析失败都返回空表（不报错），
// 让首次运行无副作用。
func loadStoreRegistry() *StoreRegistry {
	r := &StoreRegistry{
		entries: map[string]StoreEntry{},
		path:    StoreRegistryPath(),
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

// GetStoreRegistry 返回全局单例注册表，首次调用时懒加载磁盘数据（sync.Once 保证只加载一次）。
func GetStoreRegistry() *StoreRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = loadStoreRegistry()
	})
	return globalRegistry
}

// DisplayName 返回门店昵称；没注册或昵称为空时返回 fallback（通常是门店官方名/ID）。
func (r *StoreRegistry) DisplayName(storeID, fallback string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.entries[storeID]; ok && e.Nickname != "" {
		return e.Nickname
	}
	return fallback
}

// Add 新增/更新门店别名并立即落盘。
func (r *StoreRegistry) Add(id, nickname string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[id] = StoreEntry{ID: id, Nickname: nickname}
	r.save()
}

// Remove 删除门店别名并立即落盘。
func (r *StoreRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, id)
	r.save()
}

// List 返回所有别名记录的快照副本（避免外部修改内部 map）。调用方拿到的是独立切片。
func (r *StoreRegistry) List() []StoreEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]StoreEntry, 0, len(r.entries))
	for _, e := range r.entries {
		result = append(result, e)
	}
	return result
}

// save 把当前 entries 落盘。必须在持有写锁的情况下调用（被 Add/Remove 复用）。
// 文件权限 0600：stores.json 可能含可识别用户的别名，限制只属主可读写。
func (r *StoreRegistry) save() {
	entries := make([]StoreEntry, 0, len(r.entries))
	for _, e := range r.entries {
		entries = append(entries, e)
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.MkdirAll(AppDirPath(), 0o755)
	_ = os.WriteFile(r.path, data, 0o600)
}
