package forge

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"cube/util/git"
	"cube/util/gitapi"
)

// LocalRepo 对账用的本地仓库快照（取自 projcache，由调用方从 project 领域映射）。
type LocalRepo struct {
	Name    string `json:"name"`    // 项目名
	Path    string `json:"path"`    // 项目绝对路径
	RepoUrl string `json:"repoUrl"` // origin remote URL
	Dirty   bool   `json:"dirty"`   // 工作区有改动
	Ahead   int    `json:"ahead"`   // 领先远端 commit 数
	Behind  int    `json:"behind"`  // 落后远端 commit 数
}

// RepoPair 已 clone：本地与远端按 clone URL 匹配成功的一对。
type RepoPair struct {
	Local  LocalRepo         `json:"local"`
	Remote gitapi.RemoteRepo `json:"remote"`
}

// ReconcileResult 远端仓库与本地项目的对账三分结果（纯数据，纯函数产出）。
type ReconcileResult struct {
	Missing []gitapi.RemoteRepo `json:"missing"` // 未 clone 的远端库
	Orphan  []LocalRepo         `json:"orphan"`  // 本地孤儿（远端已无）
	Synced  []RepoPair          `json:"synced"`  // 已 clone
}

// RepoKey 把 repoUrl 归一化成对账匹配键：小写 host + 小写 path（去 .git 尾缀与斜杠）。
// 对账两端（远端 clone URL / 本地 remote URL）同走此函数，归一到同一口径再比对；
// 解析失败返回空串（调用方按「不参与匹配」处理）。
func RepoKey(repoUrl string) string {
	u, err := git.ParseRepoUrl(repoUrl)
	if err != nil || u == nil || u.Host == "" || u.Path == "" {
		return ""
	}
	path := strings.ToLower(strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git"))
	if path == "" {
		return ""
	}
	return NormalizeHost(u.Host) + "/" + path
}

// RepoNamespacePath 取 repoUrl 的 namespace 段（path 首段，小写），解析失败返回空串。
func RepoNamespacePath(repoUrl string) string {
	key := RepoKey(repoUrl)
	if key == "" {
		return ""
	}
	_, rest, _ := strings.Cut(key, "/") // 去掉 host 段
	ns, _, _ := strings.Cut(rest, "/")
	return ns
}

// Reconcile 对账纯函数：远端仓库列表 vs 本地仓库快照，按 RepoKey 匹配产出三类。
func Reconcile(local []LocalRepo, remote []gitapi.RemoteRepo) ReconcileResult {
	remoteByKey := make(map[string]gitapi.RemoteRepo, len(remote))
	for _, r := range remote {
		if k := RepoKey(r.CloneUrl); k != "" {
			remoteByKey[k] = r
		}
	}
	localByKey := make(map[string]LocalRepo, len(local))
	for _, l := range local {
		if k := RepoKey(l.RepoUrl); k != "" {
			localByKey[k] = l
		}
	}

	var result ReconcileResult
	for k, r := range remoteByKey {
		l, ok := localByKey[k]
		if !ok {
			result.Missing = append(result.Missing, r)
			continue
		}
		result.Synced = append(result.Synced, RepoPair{Local: l, Remote: r})
		delete(localByKey, k)
	}
	for _, l := range localByKey {
		result.Orphan = append(result.Orphan, l)
	}
	// map 遍历序随机，按名称排稳定序（展示与测试都受益）
	sortRepos(result.Missing)
	sortLocals(result.Orphan)
	sortPairs(result.Synced)
	return result
}

// --- 排序辅助（按名称稳定序） ---

func sortRepos(rs []gitapi.RemoteRepo) {
	slices.SortFunc(rs, func(a, b gitapi.RemoteRepo) int { return strings.Compare(a.Name, b.Name) })
}

func sortLocals(ls []LocalRepo) {
	slices.SortFunc(ls, func(a, b LocalRepo) int { return strings.Compare(a.Name, b.Name) })
}

func sortPairs(ps []RepoPair) {
	slices.SortFunc(ps, func(a, b RepoPair) int { return strings.Compare(a.Local.Name, b.Local.Name) })
}

// fetchRepos 实际执行一次远端拉取（account 维度，认证端点；token 必填）。
func fetchRepos(f Forge, a Account, newClient clientFactory) ([]gitapi.RemoteRepo, error) {
	client, err := newClient(f.Kind, f.Host, a.Token)
	if err != nil {
		return nil, err
	}
	repos, err := client.ListAccountRepos(context.Background())
	if err != nil {
		return nil, fmt.Errorf("拉取账号仓库列表失败: %s@%s: %w", a.Username, a.ForgeHost, err)
	}
	return repos, nil
}
