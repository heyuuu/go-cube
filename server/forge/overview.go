package forge

import (
	"time"

	"cube/util/gitapi"
)

// Overview forge 页（1042）的聚合数据：全部 namespace 的对账行 + 拉取元信息。
// 页面浏览只消费此结构，不触发外呼；拉取/刷新是显式动作。
type Overview struct {
	Rows       []RepoRow `json:"rows"`       // 对账行（只含已拉取的 namespace）
	Namespaces []NsMeta  `json:"namespaces"` // namespace 元信息（含未拉取的，供页头提示与 fetchedAt 展示）
}

// 对账状态枚举（RepoRow.Status）。
const (
	StatusMissing = "missing" // 未 clone 的远端库
	StatusSynced  = "synced"  // 已 clone
	StatusOrphan  = "orphan"  // 本地孤儿（远端已无）
)

// RepoRow 一行远端仓库（orphan 行 Repo 为零值、Local 非 nil；其余行反之）。
type RepoRow struct {
	ForgeHost string            `json:"forgeHost"`       // 所属 forge host
	NsPath    string            `json:"nsPath"`          // 所属 namespace path
	Status    string            `json:"status"`          // missing / synced / orphan
	Repo      gitapi.RemoteRepo `json:"repo"`            // 远端仓库（orphan 行为零值）
	Local     *LocalRepo        `json:"local,omitempty"` // 本地项目（missing 行为 nil）
}

// NsMeta namespace 的拉取元信息（页头 freshness 提示用）。
type NsMeta struct {
	ForgeHost string               `json:"forgeHost"`
	Path      string               `json:"path"`
	Type      gitapi.NamespaceType `json:"type"`
	FetchedAt time.Time            `json:"fetchedAt"` // 零值 = 从未拉取
	RepoCount int                  `json:"repoCount"` // 缓存中的仓库数（未拉取为 0）
}

// buildOverview 聚合纯函数：按 namespace 配置序逐个对账，行平铺；孤儿跨 namespace 去重
// （本地库归属首个命中的 namespace，重叠 namespace 不产生重复行）。
func buildOverview(namespaces []Namespace, lookup func(Namespace) (NsCacheEntry, bool), local []LocalRepo) *Overview {
	ov := &Overview{Namespaces: make([]NsMeta, 0, len(namespaces))}
	claimed := make(map[string]bool, len(local)) // 已归属某 namespace 的本地 RepoKey
	for _, ns := range namespaces {
		meta := NsMeta{ForgeHost: ns.ForgeHost, Path: ns.Path, Type: ns.Type}
		entry, ok := lookup(ns)
		if ok {
			meta.FetchedAt = entry.FetchedAt
			meta.RepoCount = len(entry.Repos)
			ov.appendRows(ns, entry.Repos, local, claimed)
		}
		ov.Namespaces = append(ov.Namespaces, meta)
	}
	return ov
}

// appendRows 把单个 namespace 的对账三分结果平铺进行集。
func (ov *Overview) appendRows(ns Namespace, repos []gitapi.RemoteRepo, local []LocalRepo, claimed map[string]bool) {
	var scoped []LocalRepo
	for _, l := range local {
		if !claimed[RepoKey(l.RepoUrl)] && MatchNamespacePath(l.RepoUrl, ns.ForgeHost, ns.Path) {
			scoped = append(scoped, l)
		}
	}
	result := Reconcile(scoped, repos)
	for _, r := range result.Missing {
		ov.Rows = append(ov.Rows, RepoRow{ForgeHost: ns.ForgeHost, NsPath: ns.Path, Status: StatusMissing, Repo: r})
	}
	for _, pair := range result.Synced {
		local := pair.Local
		claimed[RepoKey(local.RepoUrl)] = true
		ov.Rows = append(ov.Rows, RepoRow{ForgeHost: ns.ForgeHost, NsPath: ns.Path, Status: StatusSynced, Repo: pair.Remote, Local: &local})
	}
	// 孤儿：本 namespace 范围内、未被任何 namespace 认领且不命中任何远端库的本地库。
	// 注意 Reconcile 的 orphan = scoped - synced，但 scoped 已排除被前面 namespace 认领的，
	// 且同 namespace 内 matched 的已从 localByKey 剔除——此处 orphan 即新认领。
	for _, l := range result.Orphan {
		claimed[RepoKey(l.RepoUrl)] = true
		ov.Rows = append(ov.Rows, RepoRow{ForgeHost: ns.ForgeHost, NsPath: ns.Path, Status: StatusOrphan, Local: &l})
	}
}
