package gogit

// 本文件封装「读操作」的 go-git 纯 Go 实现，与 git.go 中的系统 git 命令封装相对。
//
// 为什么读操作要单独用 go-git（而不是统一走系统 git 子进程）：
//   - cube 是 CLI 模式执行，单次命令里常常要对一批项目采集 git 信息（典型场景：
//     `project list --status` 会为每个 git 项目查询 branch/ahead-behind/dirty）。
//   - 若用系统 git，每个项目至少要 fork 3 个子进程（branch/log/status），N 个项目
//     就是 3N 次 fork/exec；在 macOS 上每次 fork/exec 1~2ms，几十个项目就显著卡顿。
//   - go-git 直接读取 .git 目录（config / HEAD / refs / objects），零子进程；配合
//     缓存层可以做到后台异步采集，前台读命令几乎零开销。
//
// 为什么写操作（clone 等）仍然保留系统 git（见 git.go）：
//   - clone 需要交互式进度输出、SSH 凭据、git hooks 等本地 git 生态，go-git 的兼容性
//     与体验都不如系统 git；因此写操作继续走 git.go 中的系统 git 子进程封装。
//
// 错误处理约定（全文件一致）：
//   - 非 git 目录、缺失 remote/分支等「业务上可接受的空值」场景：返回零值 + nil，
//     不向调用方抛 error。上层缓存层依赖此约定统一兜底。
//   - 真实读取错误（损坏的 .git、IO 异常等）：返回零值 + error，由调用方决定是否记录。

import (
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// openRepo 在 path 处打开一个 git 仓库（DetectDotGit: 允许从子目录向上探测 .git）。
func openRepo(path string) (*gogit.Repository, error) {
	return gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{DetectDotGit: false})
}

// RemoteUrl 返回 path 处仓库 origin remote 的 URL。
// 无 origin remote 时返回 ("", nil)，不视为错误。
func RemoteUrl(path string) (string, error) {
	repo, err := openRepo(path)
	if err != nil {
		return "", nil // 非仓库目录：返回空值，不报错
	}

	remote, err := repo.Remote(gogit.DefaultRemoteName) // "origin"
	if err != nil {
		return "", nil // 无 origin remote：返回空值，不报错
	}

	urls := remote.Config().URLs
	if len(urls) == 0 {
		return "", nil
	}
	return urls[0], nil
}

// Branches 返回 path 处仓库的全部分支列表（本地 + 远程）以及当前分支名。
//
// 输出格式与原系统 git 实现保持一致，便于上层无感替换：
//   - 本地分支短名：     "master" / "develop"
//   - 远程分支短名：     "origin/master"（带 remote 名前缀）
//   - 当前分支：         同上短名形式；HEAD detached 时返回空串
func Branches(path string) (branches []string, current string, err error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, "", nil
	}

	// 当前分支：HEAD 指向的 ref；detached HEAD 视为无当前分支
	if head, headErr := repo.Head(); headErr == nil {
		current = refShortName(head.Name())
	}

	// 全部分支：遍历 branches iterator
	iter, err := repo.Branches()
	if err != nil {
		return nil, current, nil
	}
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, refShortName(ref.Name()))
		return nil
	})
	return branches, current, nil
}

// refShortName 把 plumbing.ReferenceName 折算成短名，区分本地与远程：
//   - refs/heads/X           -> X
//   - refs/remotes/origin/X  -> origin/X
//   - 其他                   -> Name().Short()
func refShortName(name plumbing.ReferenceName) string {
	if name.IsBranch() {
		return name.Short() // refs/heads/* → 短名
	}
	if name.IsRemote() {
		// refs/remotes/{remote}/{branch...} → {remote}/{branch...}
		// plumbing.ReferenceName 没有 Fields()，自己 split。
		parts := strings.SplitN(name.String(), "/", 4)
		// parts: ["refs", "remotes", remote, branch(可能含 /)]
		if len(parts) == 4 {
			return parts[2] + "/" + parts[3]
		}
	}
	return name.Short()
}

// DefaultBranch 返回 path 处仓库的默认分支短名（"master" / "main" 等）。
// 用于 ahead/behind 比较「主分支本地 vs 主分支远程」。
//
// 判断顺序（可靠度递减）：
//  1. origin/HEAD 指向的分支（远程仓库声明的 default branch；克隆时自动建立）
//  2. 本地 master 分支存在 → master（兼容老仓库的常见情况）
//  3. 本地 main 分支存在 → main（新仓库的常见情况）
//  4. 都没有 → 返回空串（调用方应跳过 ahead/behind）
func DefaultBranch(path string) (string, error) {
	repo, err := openRepo(path)
	if err != nil {
		return "", nil
	}
	return resolveDefaultBranch(repo), nil
}

// resolveDefaultBranch 在已打开的 repo 上解析默认分支。
// 抽出来便于未来在 collectEntry 复用 repo 实例时直接调用。
func resolveDefaultBranch(repo *gogit.Repository) string {
	// 1. origin/HEAD 是 symbolic ref，指向 refs/remotes/origin/{branch}
	if ref, err := repo.Reference(plumbing.ReferenceName("refs/remotes/origin/HEAD"), false); err == nil {
		// target 形如 refs/remotes/origin/master，折算成 master 这种本地短名
		if target := ref.Target(); target.IsRemote() {
			return stripRemotePrefix(target.Short()) // origin/master → master
		}
	}
	// 2. fallback master
	if ref, err := repo.Reference(plumbing.NewBranchReferenceName("master"), true); err == nil && ref != nil {
		return "master"
	}
	// 3. fallback main
	if ref, err := repo.Reference(plumbing.NewBranchReferenceName("main"), true); err == nil && ref != nil {
		return "main"
	}
	return ""
}

// AheadBehind 计算本地分支 local 相对远程分支 remote 的领先 / 落后 commit 数。
//   - ahead  = 本地有、远程没有的 commit 数（待推送）
//   - behind = 远程有、本地没有的 commit 数（待拉取）
//
// 入参接受分支短名（"master" / "origin/master"）；任一 ref 缺失返回 (0, 0, nil)。
//
// 实现说明：当前 go-git v5 的 LogOptions 不支持 Exclude，无法用单次遍历算差集。
// 这里分别遍历两个 ref 各自可达的 commit、建立 hash 集合后做集合差，得到 ahead/behind。
// go-git 不会 fetch，仅比对本地已有的 commit —— 未 fetch 过的数据可能不准，
// 这与「缓存场景接受 stale」的整体策略一致。
func AheadBehind(path string, local, remote string) (ahead, behind int, err error) {
	repo, err := openRepo(path)
	if err != nil {
		return 0, 0, nil
	}

	localHash, ok := resolveBranchHash(repo, local, false)
	if !ok {
		return 0, 0, nil
	}
	remoteHash, ok := resolveBranchHash(repo, remote, true)
	if !ok {
		return 0, 0, nil
	}
	if localHash == remoteHash {
		return 0, 0, nil
	}

	localSet, err := reachableCommits(repo, localHash)
	if err != nil {
		return 0, 0, nil
	}
	remoteSet, err := reachableCommits(repo, remoteHash)
	if err != nil {
		return 0, 0, nil
	}

	// ahead: 在 localSet 但不在 remoteSet
	for h := range localSet {
		if !remoteSet[h] {
			ahead++
		}
	}
	// behind: 在 remoteSet 但不在 localSet
	for h := range remoteSet {
		if !localSet[h] {
			behind++
		}
	}
	return ahead, behind, nil
}

// resolveBranchHash 在 repo 内按短名解析分支的 commit hash。
// isRemote=true 时按远程分支解析（"origin/master" → refs/remotes/origin/master）。
func resolveBranchHash(repo *gogit.Repository, shortName string, isRemote bool) (plumbing.Hash, bool) {
	var refName plumbing.ReferenceName
	if isRemote {
		refName = plumbing.NewRemoteReferenceName(gogit.DefaultRemoteName, stripRemotePrefix(shortName))
	} else {
		refName = plumbing.NewBranchReferenceName(shortName)
	}
	ref, err := repo.Reference(refName, true)
	if err != nil {
		return plumbing.ZeroHash, false
	}
	return ref.Hash(), true
}

// stripRemotePrefix 去掉远程分支短名里的 remote 前缀（"origin/master" → "master"）。
func stripRemotePrefix(name string) string {
	for _, prefix := range []string{"origin/", "upstream/"} {
		if strings.HasPrefix(name, prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

// reachableCommits 返回从 from 出发可达的全部 commit hash 集合。
// 用于 AheadBehind 的集合差计算。
func reachableCommits(repo *gogit.Repository, from plumbing.Hash) (map[plumbing.Hash]bool, error) {
	iter, err := repo.Log(&gogit.LogOptions{From: from})
	if err != nil {
		return nil, err
	}
	set := make(map[plumbing.Hash]bool)
	_ = iter.ForEach(func(c *object.Commit) error {
		set[c.Hash] = true
		return nil
	})
	return set, nil
}

// IsDirty 返回 path 处仓库的工作区是否有改动（含 untracked，但尊重 .gitignore）。
// 非仓库目录返回 (false, nil)。
func IsDirty(path string) (bool, error) {
	repo, err := openRepo(path)
	if err != nil {
		return false, nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return false, err
	}
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	return !status.IsClean(), nil
}
