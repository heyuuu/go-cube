package gogit

// 本包封装「读操作」的 go-git 纯 Go 实现，与 git 包中的系统 git 命令封装相对。
//
// 为什么读操作要单独用 go-git（而不是统一走系统 git 子进程）：
//   - cube 是 CLI 模式执行，单次命令里常常要对一批项目采集 git 信息（典型场景：
//     `project list --status` 会为每个 git 项目查询 branch/ahead-behind/dirty）。
//   - 若用系统 git，每个项目至少要 fork 3 个子进程（branch/log/status），N 个项目
//     就是 3N 次 fork/exec；在 macOS 上每次 fork/exec 1~2ms，几十个项目就显著卡顿。
//   - go-git 直接读取 .git 目录（config / HEAD / refs / objects），零子进程；配合
//     缓存层可以做到后台异步采集，前台读命令几乎零开销。
//
// 写操作（clone / init / commit 等）以及与具体 git 库无关的 git 辅助能力
// （仓库根探测、repoUrl 解析等）见独立的 git 包（util/git）。
//
// 错误处理约定（全包一致）：
//   - 非 git 目录、缺失 remote/分支等「业务上可接受的空值」场景：返回零值 + nil，
//     不向调用方抛 error。上层缓存层依赖此约定统一兜底。
//   - 真实读取错误（损坏的 .git、IO 异常等）：返回零值 + error，由调用方决定是否记录。

import (
	"sort"
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

// Remote 描述一个 remote：名字 + 抓取/推送地址（取各自的第一条）。
type Remote struct {
	Name  string
	Fetch string
	Push  string
}

// Remotes 返回 path 处仓库的全部 remote（按名字排序）。
// 每个 remote 的 Fetch / Push 取其配置 URLs 的第一条（多数 remote 只有 push==fetch 一条）。
// 非仓库目录或无任何 remote 时返回 (nil, nil)，不视为错误。
func Remotes(path string) ([]Remote, error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, nil
	}

	remotes, err := repo.Remotes()
	if err != nil {
		return nil, nil
	}

	var result []Remote
	for _, r := range remotes {
		cfg := r.Config()
		// go-git 的 RemoteConfig 不区分 fetch/push URL 列表，统一放在 URLs；
		// 多数 remote 只有一条（既抓又推），这里第二条（若有）当作 push 专用地址展示。
		var fetch, push string
		if len(cfg.URLs) >= 2 {
			fetch, push = cfg.URLs[0], cfg.URLs[1]
		} else if len(cfg.URLs) >= 1 {
			fetch, push = cfg.URLs[0], cfg.URLs[0]
		}
		result = append(result, Remote{Name: cfg.Name, Fetch: fetch, Push: push})
	}
	// 按名字排序，保证输出稳定
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// Branches 返回 path 处仓库的全部本地分支列表以及当前分支名。
//
// 输出格式：
//   - 本地分支短名：     "master" / "develop"（仅 refs/heads/*，不含远程分支）
//   - 当前分支：         同上短名形式；HEAD detached 时返回空串
//
// 注意：go-git 的 repo.Branches() 只遍历 refs/heads/*，远程分支（refs/remotes/*）
// 需用 RemoteBranches 获取。
func Branches(path string) (branches []string, current string, err error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, "", nil
	}

	// 当前分支：HEAD 指向的 ref；detached HEAD 视为无当前分支
	if head, headErr := repo.Head(); headErr == nil {
		current = refShortName(head.Name())
	}

	// 全部本地分支：遍历 branches iterator（仅 refs/heads/*）
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

// RemoteBranch 描述一个远程分支：所属 remote 名 + 分支名（不含 remote 前缀）。
// 例 origin/master → {Remote:"origin", Branch:"master"}。
type RemoteBranch struct {
	Remote string
	Branch string
}

// RemoteBranches 返回 path 处仓库的全部远程分支（所有 remote 的 refs/remotes/*）。
//
// 自动跳过各 remote 的 HEAD（refs/remotes/{remote}/HEAD，它是 symbolic ref 而非真实分支）。
// 非仓库目录或无任何远程分支时返回 (nil, nil)，不视为错误。
func RemoteBranches(path string) ([]RemoteBranch, error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, nil
	}
	iter, err := repo.References()
	if err != nil {
		return nil, nil
	}
	var result []RemoteBranch
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name()
		if !name.IsRemote() {
			return nil
		}
		remote, branch, ok := splitRemoteRef(name)
		if !ok {
			return nil
		}
		result = append(result, RemoteBranch{Remote: remote, Branch: branch})
		return nil
	})
	return result, nil
}

// Tags 返回 path 处仓库的全部 tag 名（按名字升序，含轻量 tag 与 annotated tag）。
// 非仓库目录或无 tag 时返回 (nil, nil)，不视为错误。
func Tags(path string) ([]string, error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, nil
	}
	iter, err := repo.Tags()
	if err != nil {
		return nil, nil
	}
	var tags []string
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		// refs/tags/<name> → <name>；Short() 已等价处理但显式裁前缀更直观
		tags = append(tags, strings.TrimPrefix(ref.Name().String(), "refs/tags/"))
		return nil
	})
	sort.Strings(tags)
	return tags, nil
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
		remote, branch, ok := splitRemoteRef(name)
		if ok {
			return remote + "/" + branch
		}
	}
	return name.Short()
}

// splitRemoteRef 把 refs/remotes/{remote}/{branch...} 拆成 (remote, branch)。
// 入参必须是远程引用（IsRemote()==true）；HEAD 这种 symbolic ref 会返回 ok=false。
func splitRemoteRef(name plumbing.ReferenceName) (remote, branch string, ok bool) {
	// parts: ["refs", "remotes", remote, branch(可能含 /)]
	parts := strings.SplitN(name.String(), "/", 4)
	if len(parts) != 4 || parts[3] == "HEAD" {
		return "", "", false
	}
	return parts[2], parts[3], true
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

// AheadBehindRemote 计算本地分支 localBranch 相对指定 remote 的 remoteBranch 的领先 / 落后数。
// 与 AheadBehind 的区别：显式接受 remote 名，便于比较非 origin 的远程分支。
// 任一 ref 缺失（如该 remote 没有这个分支）返回 (0, 0, nil)，不视为错误。
//
// 与 AheadBehind 一样基于本地已有 commit 比对，不会 fetch。
func AheadBehindRemote(path string, localBranch, remoteName, remoteBranch string) (ahead, behind int, err error) {
	repo, err := openRepo(path)
	if err != nil {
		return 0, 0, nil
	}

	localHash, ok := resolveBranchHash(repo, localBranch, false)
	if !ok {
		return 0, 0, nil
	}
	remoteRefName := plumbing.NewRemoteReferenceName(remoteName, remoteBranch)
	remoteRef, err := repo.Reference(remoteRefName, true)
	if err != nil {
		return 0, 0, nil
	}
	remoteHash := remoteRef.Hash()
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
	for h := range localSet {
		if !remoteSet[h] {
			ahead++
		}
	}
	for h := range remoteSet {
		if !localSet[h] {
			behind++
		}
	}
	return ahead, behind, nil
}

// resolveBranchHash 在 repo 内按短名解析分支的 commit hash。
//   - isRemote=false：按本地分支解析（"master" → refs/heads/master）
//   - isRemote=true：按远程分支解析（"origin/master" → refs/remotes/origin/master），
//     remote 名取自 shortName 的前缀（stripRemotePrefix 反向操作）
func resolveBranchHash(repo *gogit.Repository, shortName string, isRemote bool) (plumbing.Hash, bool) {
	var refName plumbing.ReferenceName
	if isRemote {
		remote, branch, ok := splitRemoteBranchShortName(shortName)
		if !ok {
			return plumbing.ZeroHash, false
		}
		refName = plumbing.NewRemoteReferenceName(remote, branch)
	} else {
		refName = plumbing.NewBranchReferenceName(shortName)
	}
	ref, err := repo.Reference(refName, true)
	if err != nil {
		return plumbing.ZeroHash, false
	}
	return ref.Hash(), true
}

// splitRemoteBranchShortName 把 "origin/master" 这种远程分支短名拆成 (origin, master)。
// 支持分支名含 /（如 "origin/feature/x" → ("origin", "feature/x")）。
// 无 remote 前缀时返回 ok=false。
func splitRemoteBranchShortName(shortName string) (remote, branch string, ok bool) {
	idx := strings.Index(shortName, "/")
	if idx <= 0 {
		return "", "", false
	}
	return shortName[:idx], shortName[idx+1:], true
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

// FileStatus 单个文件的工作区状态，展示形态对齐 git status --short。
type FileStatus struct {
	Code string // 两字符状态码 XY（X=暂存区，Y=工作区），如 "M " / " M" / "??" / "A " / "D "
	Path string // 相对仓库根路径
}

// StatusFiles 返回 path 处仓库工作区有变动的文件列表（含 untracked，尊重 .gitignore），
// 按路径排序。非仓库目录或工作区干净时返回 (nil, nil)，不视为错误。
//
// 注意：go-git 不做 rename 检测，改名文件表现为「旧路径 D + 新路径 A」两行
// （git status --short 会合并显示为 R 行，此处不合并）。
func StatusFiles(path string) ([]FileStatus, error) {
	repo, err := openRepo(path)
	if err != nil {
		return nil, nil
	}
	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}

	var files []FileStatus
	for path, fs := range status {
		// 两列都是空格（unmodified）的行不展示，与 git status --short 行为一致
		if fs.Staging == gogit.Unmodified && fs.Worktree == gogit.Unmodified {
			continue
		}
		files = append(files, FileStatus{
			Code: string(rune(fs.Staging)) + string(rune(fs.Worktree)),
			Path: path,
		})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}
