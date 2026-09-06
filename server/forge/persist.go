package forge

import (
	"time"

	"cube/util/gitapi"
	"cube/util/store"
)

// namespace 拉取结果的持久化形态（cache/forge-repos.json，提案 1042）。
// 纯缓存语义：可整体删除且行为不变差（重拉即重建）；落盘是为了 forge 页在 server
// 重启后仍有数据可看——拉取是显式动作，重启即空会让「数据过期而不自知」。
// fetchedAt 随数据落盘，前端据此展示新鲜度。

// NsCacheEntry 单个 namespace 的拉取结果快照。
type NsCacheEntry struct {
	FetchedAt time.Time           `json:"fetchedAt"` // 最近一次成功拉取时间（零值不出现：只有成功才落盘）
	Repos     []gitapi.RemoteRepo `json:"repos"`     // 该 namespace 下的远端仓库全量
}

// nsDiskCache 落盘文档形态：nsCacheKey(host/path) → 快照。
type nsDiskCache struct {
	Namespaces map[string]NsCacheEntry `json:"namespaces"`
}

// loadDiskCache 读落盘缓存；文件不存在/坏 JSON 统一降级为空缓存（读路径不阻断）。
func loadDiskCache(file string) nsDiskCache {
	d, err := store.LoadJson[nsDiskCache](file)
	if err != nil {
		return nsDiskCache{Namespaces: map[string]NsCacheEntry{}}
	}
	if d.Namespaces == nil {
		d.Namespaces = map[string]NsCacheEntry{}
	}
	return d
}

// saveDiskCache 原子写落盘缓存。
func saveDiskCache(file string, d nsDiskCache) error {
	if err := store.SaveJson(file, d); err != nil {
		return err
	}
	return nil
}
