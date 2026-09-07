package forge

import (
	"time"

	"cube/util/gitapi"
	"cube/util/store"
)

// account 拉取结果的持久化形态（cache/forge-repos.json，提案 1042，1044 改按 account 键）。
// 纯缓存语义：可整体删除且行为不变差（重拉即重建）；落盘是为了 forge 页在 server
// 重启后仍有数据可看——拉取是显式动作，重启即空会让「数据过期而不自知」。
// fetchedAt 随数据落盘，前端据此展示新鲜度。

// AcctCacheEntry 单个 account 的拉取结果快照。
type AcctCacheEntry struct {
	FetchedAt time.Time           `json:"fetchedAt"` // 最近一次成功拉取时间（零值不出现：只有成功才落盘）
	Repos     []gitapi.RemoteRepo `json:"repos"`     // 该账号名下的远端仓库全量
}

// acctDiskCache 落盘文档形态：acctCacheKey(host/username) → 快照。
type acctDiskCache struct {
	Accounts map[string]AcctCacheEntry `json:"accounts"`
}

// loadDiskCache 读落盘缓存；文件不存在/坏 JSON 统一降级为空缓存（读路径不阻断）。
func loadDiskCache(file string) acctDiskCache {
	d, err := store.LoadJson[acctDiskCache](file)
	if err != nil {
		return acctDiskCache{Accounts: map[string]AcctCacheEntry{}}
	}
	if d.Accounts == nil {
		d.Accounts = map[string]AcctCacheEntry{}
	}
	return d
}

// saveDiskCache 原子写落盘缓存。
func saveDiskCache(file string, d acctDiskCache) error {
	return store.SaveJson(file, d)
}
