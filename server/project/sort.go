package project

import (
	"slices"
	"time"
)

// SortByRecentUsage 最近使用的项目置顶（按最近使用倒序，最多 limit 条），其余保持原序。
// CLI 与 alfred 出口共用的一分实现；Web 侧排序在前端做（list 接口返回原始序 + lastUsedAt）。
// latest 是 usage 读侧的 path → 最近使用时间；以 map 传参、纯函数实现，project 包不依赖 usage。
func SortByRecentUsage(projects []*Project, latest map[string]time.Time, limit int) []*Project {
	pinned := make([]string, 0, len(latest))
	for path := range latest {
		pinned = append(pinned, path)
	}
	slices.SortFunc(pinned, func(a, b string) int {
		return latest[b].Compare(latest[a])
	})
	if len(pinned) > limit {
		pinned = pinned[:limit]
	}

	weights := make(map[string]int, len(projects))
	for i, proj := range projects {
		weights[proj.Path()] = i + len(pinned)
	}
	for i, path := range pinned {
		weights[path] = i
	}

	slices.SortFunc(projects, func(a, b *Project) int {
		return weights[a.Path()] - weights[b.Path()]
	})

	return projects
}
