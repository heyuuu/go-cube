package forge

import (
	"strings"
	"time"

	"cube/util/gitapi"
)

// Overview forge 页（1042/1044）的聚合数据：全部 account 的对账行 + 拉取元信息。
// 页面浏览只消费此结构，不触发外呼；拉取/刷新是显式动作。
type Overview struct {
	Rows     []RepoRow  `json:"rows"`     // 对账行（只含已拉取的 account）
	Accounts []AcctMeta `json:"accounts"` // account 元信息（含未拉取的，供页头提示与 fetchedAt 展示）
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
	Owner     string            `json:"owner"`           // 仓库归属命名空间（从 URL path 推导，如 heyuuu / ce_lbt），纯展示维度
	Status    string            `json:"status"`          // missing / synced / orphan
	Repo      gitapi.RemoteRepo `json:"repo"`            // 远端仓库（orphan 行为零值）
	Local     *LocalRepo        `json:"local,omitempty"` // 本地项目（missing 行为 nil）
}

// AcctMeta account 的拉取元信息（页头 freshness 提示用）。
type AcctMeta struct {
	ForgeHost string    `json:"forgeHost"`
	Username  string    `json:"username"`
	FetchedAt time.Time `json:"fetchedAt"` // 零值 = 从未拉取
	RepoCount int       `json:"repoCount"` // 缓存中的仓库数（未拉取为 0）
}

// buildOverview 聚合纯函数：按 account 配置序逐个对账，行平铺（同 forge 多账号取并集去重）。
//
// 孤儿定义（1044）：本地 remote 与该 forge 拉取到的仓库同 namespace（URL path 首段）、
// 但不在拉取结果里的项目——「同空间但远端已看不到」，大概率是远端已删/已转移。
// namespace 不再是配置，全部从 repoUrl / full_name 推导。
func buildOverview(forges []Forge, accounts []Account, lookup func(Account) (AcctCacheEntry, bool), local []LocalRepo) *Overview {
	ov := &Overview{Accounts: make([]AcctMeta, 0, len(accounts))}
	// forge host → 该 forge 全部账号的拉取结果（并集）
	fetchedByForge := map[string][]gitapi.RemoteRepo{}
	forgeByHost := map[string]Forge{}
	for _, f := range forges {
		forgeByHost[NormalizeHost(f.Host)] = f
	}
	for _, a := range accounts {
		a.ForgeHost = NormalizeHost(a.ForgeHost)
		meta := AcctMeta{ForgeHost: a.ForgeHost, Username: a.Username}
		if entry, ok := lookup(a); ok {
			meta.FetchedAt = entry.FetchedAt
			meta.RepoCount = len(entry.Repos)
			fetchedByForge[a.ForgeHost] = append(fetchedByForge[a.ForgeHost], entry.Repos...)
		}
		ov.Accounts = append(ov.Accounts, meta)
	}

	claimed := make(map[string]bool, len(local)) // 已判定（synced/orphan）的本地 RepoKey
	for host, repos := range fetchedByForge {
		// 命名空间全集 = 该 forge 拉到的仓库的 full_name / URL path 首段
		nsSet := map[string]bool{}
		for _, r := range repos {
			if ns, _, ok := strings.Cut(r.FullName, "/"); ok && ns != "" {
				nsSet[strings.ToLower(ns)] = true
			} else if ns := RepoNamespacePath(r.CloneUrl); ns != "" {
				nsSet[ns] = true
			}
		}
		// 三分：synced / missing 由 Reconcile 产出；orphan 单独按新定义算
		result := Reconcile(local, repos)
		for _, r := range result.Missing {
			ov.Rows = append(ov.Rows, RepoRow{
				ForgeHost: host, Owner: repoOwner(r.FullName, r.CloneUrl),
				Status: StatusMissing, Repo: r,
			})
		}
		for _, pair := range result.Synced {
			local := pair.Local
			claimed[RepoKey(local.RepoUrl)] = true
			ov.Rows = append(ov.Rows, RepoRow{
				ForgeHost: host, Owner: repoOwner(pair.Remote.FullName, pair.Remote.CloneUrl),
				Status: StatusSynced, Repo: pair.Remote, Local: &local,
			})
		}
		// 孤儿：host 命中该 forge、namespace 在拉取结果的全集里、但仓库本身不在其中
		for _, l := range local {
			key := RepoKey(l.RepoUrl)
			if key == "" || claimed[key] || !strings.EqualFold(RepoHost(l.RepoUrl), host) {
				continue
			}
			ns := RepoNamespacePath(l.RepoUrl)
			if ns == "" || !nsSet[ns] {
				continue
			}
			claimed[key] = true
			ov.Rows = append(ov.Rows, RepoRow{ForgeHost: host, Owner: ns, Status: StatusOrphan, Local: &l})
		}
	}
	return ov
}

// repoOwner 从 full_name 或 cloneUrl 推导 owner（namespace）段。
func repoOwner(fullName, cloneUrl string) string {
	if ns, _, ok := strings.Cut(fullName, "/"); ok && ns != "" {
		return ns
	}
	return RepoNamespacePath(cloneUrl)
}
