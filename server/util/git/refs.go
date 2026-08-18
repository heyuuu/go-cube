package git

// ref 查询：remote / 本地与远程分支 / tag / 默认分支 / HEAD 与父提交 / ahead-behind。
// 执行核与错误约定见 run.go。

import (
	"sort"
	"strconv"
	"strings"
)

// Remote 描述一个 remote：名字 + 抓取/推送地址（取各自的第一条）。
type Remote struct {
	Name  string
	Fetch string
	Push  string
}

// Remotes 返回 path 处仓库的全部 remote（按名字排序）。
// Fetch / Push 取各自的第一条地址；配置了独立 pushurl 的 remote 两者不同。
// 非仓库目录或无任何 remote 时返回 (nil, nil)，不视为错误。
func Remotes(path string) ([]Remote, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, "remote", "-v")
	if err != nil {
		return nil, err
	}
	return parseRemotesVerbose(out), nil
}

// parseRemotesVerbose 解析 `git remote -v` 输出，每行形如（name 与 url 间是 TAB）：
//
//	origin	git@github.com:a/b.git (fetch)
//	origin	git@github.com:a/b.git (push)
//
// 同一 remote 的 fetch/push 行合并为一条，同名多行取第一条（多 URL remote 属罕见配置）。
func parseRemotesVerbose(out string) []Remote {
	index := make(map[string]*Remote)
	for _, line := range strings.Split(out, "\n") {
		name, rest, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		url, field, found := strings.Cut(strings.TrimSpace(rest), " (")
		if !found || !strings.HasSuffix(field, ")") {
			continue
		}
		r, ok := index[name]
		if !ok {
			r = &Remote{Name: name}
			index[name] = r
		}
		switch strings.TrimSuffix(field, ")") {
		case "fetch":
			if r.Fetch == "" {
				r.Fetch = url
			}
		case "push":
			if r.Push == "" {
				r.Push = url
			}
		}
	}

	if len(index) == 0 {
		return nil
	}
	remotes := make([]Remote, 0, len(index))
	for _, r := range index {
		remotes = append(remotes, *r)
	}
	// 按名字排序，保证输出稳定
	sort.Slice(remotes, func(i, j int) bool { return remotes[i].Name < remotes[j].Name })
	return remotes
}

// RemoteUrl 返回 path 处仓库 origin remote 的抓取地址——Remotes 的 origin 特化查找，
// 多 URL 取第一条的口径与 Remotes 一致。无 origin remote 时返回 ("", nil)，不视为错误。
func RemoteUrl(path string) (string, error) {
	remotes, err := Remotes(path)
	if err != nil {
		return "", err
	}
	for _, r := range remotes {
		if r.Name == "origin" {
			return r.Fetch, nil
		}
	}
	return "", nil
}

// Branches 返回 path 处仓库的全部本地分支列表（仅 refs/heads/*，不含远程分支）
// 以及当前分支名。当前分支为 HEAD 指向的短名，detached 时为空串。
// 非仓库返回 (nil, "", nil)，不视为错误。
func Branches(path string) (branches []string, current string, err error) {
	if !isGitRepo(path) {
		return nil, "", nil
	}
	// symbolic-ref 失败（非 0 退出）= detached HEAD，保持空串即可
	if out, err := runOut(path, "symbolic-ref", "--short", "HEAD"); err == nil {
		current = strings.TrimSpace(out)
	}
	out, err := runOut(path, "for-each-ref", "--format=%(refname)", "refs/heads/")
	if err != nil {
		return nil, current, err
	}
	for _, ref := range strings.Split(strings.TrimSpace(out), "\n") {
		if ref != "" {
			branches = append(branches, strings.TrimPrefix(ref, "refs/heads/"))
		}
	}
	return branches, current, nil
}

// RemoteBranch 描述一个远程分支：所属 remote 名 + 分支名（不含 remote 前缀）。
// 例 origin/master → {Remote:"origin", Branch:"master"}。
type RemoteBranch struct {
	Remote string
	Branch string
}

// RemoteBranches 返回 path 处仓库的全部远程分支（所有 remote 的 refs/remotes/*）。
// 自动跳过各 remote 的 HEAD（refs/remotes/{remote}/HEAD，它是 symbolic ref 而非真实分支）。
// 非仓库目录或无任何远程分支时返回 (nil, nil)，不视为错误。
func RemoteBranches(path string) ([]RemoteBranch, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, "for-each-ref", "--format=%(refname)", "refs/remotes/")
	if err != nil {
		return nil, err
	}
	var result []RemoteBranch
	for _, ref := range strings.Split(strings.TrimSpace(out), "\n") {
		if ref == "" {
			continue
		}
		remote, branch, ok := splitRemoteBranchShortName(strings.TrimPrefix(ref, "refs/remotes/"))
		if !ok || branch == "HEAD" {
			continue
		}
		result = append(result, RemoteBranch{Remote: remote, Branch: branch})
	}
	return result, nil
}

// splitRemoteBranchShortName 把 "origin/master" 这种远程分支短名拆成 (remote, branch)。
// 支持分支名含 /（如 "origin/feature/x" → ("origin", "feature/x")）。
// 无 remote 前缀时返回 ok=false。
func splitRemoteBranchShortName(shortName string) (remote, branch string, ok bool) {
	idx := strings.Index(shortName, "/")
	if idx <= 0 {
		return "", "", false
	}
	return shortName[:idx], shortName[idx+1:], true
}

// Tags 返回 path 处仓库的全部 tag 名（按名字升序，含轻量 tag 与 annotated tag）。
// 非仓库目录或无 tag 时返回 (nil, nil)，不视为错误。
func Tags(path string) ([]string, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, "for-each-ref", "--format=%(refname)", "refs/tags/")
	if err != nil {
		return nil, err
	}
	var tags []string
	for _, ref := range strings.Split(strings.TrimSpace(out), "\n") {
		if ref != "" {
			tags = append(tags, strings.TrimPrefix(ref, "refs/tags/"))
		}
	}
	return tags, nil
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
	if !isGitRepo(path) {
		return "", nil
	}
	// origin/HEAD 未设置时 symbolic-ref 报错（非 0 退出），走 fallback
	if out, err := runOut(path, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if _, branch, ok := splitRemoteBranchShortName(strings.TrimSpace(out)); ok && branch != "HEAD" {
			return branch, nil
		}
	}
	for _, candidate := range []string{"master", "main"} {
		// show-ref --verify --quiet 只用退出码表达存在性（0=存在）
		if _, err := runOut(path, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

// HeadSha 返回 path 仓库 HEAD 指向 commit 的完整 sha（detached HEAD 同样适用）。
// 空仓库（尚无任何 commit）时 rev-parse 报错，原样返回错误。
// 只需 sha 时不要改用 LoadRepoStatus 的 Sha 字段——status 要扫描工作区，
// rev-parse 是 O(1)。
func HeadSha(path string) (string, error) {
	if !isGitRepo(path) {
		return "", nil
	}
	out, err := runOut(path, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return firstLine(out), nil
}

// ParentSha 返回 ref 的父提交完整 sha。
// 根提交没有父、ref 不存在时 git 报错并原样返回——是否降级（如改用空树）
// 由调用方决定，见 workbench.Changes。
func ParentSha(path string, ref string) (string, error) {
	out, err := runOut(path, "rev-parse", ref+"^")
	if err != nil {
		return "", err
	}
	return firstLine(out), nil
}

// AheadBehindRemote 计算本地分支 localBranch 相对指定 remote 的 remoteBranch 的
// 领先 / 落后数。
//   - ahead  = 本地有、远程没有的 commit 数（待推送）
//   - behind = 远程有、本地没有的 commit 数（待拉取）
//
// 比较基准显式拆成 remote 名 + 分支名，便于比较非 origin 的远程分支；
// gitcache 用它做「默认分支 vs origin/<默认分支>」的列表页 ahead/behind。
// 任一 ref 缺失（如该 remote 没有这个分支）返回 (0, 0, nil)，不视为错误。
// 基于本地已有 commit 比对（不 fetch），未 fetch 过的数据可能不准——
// 与「缓存场景接受 stale」的整体策略一致。
func AheadBehindRemote(path string, localBranch, remoteName, remoteBranch string) (ahead, behind int, err error) {
	return aheadBehindRefs(path, "refs/heads/"+localBranch, "refs/remotes/"+remoteName+"/"+remoteBranch)
}

// aheadBehindRefs 用 `rev-list --left-right --count left...right` 计算双方各自
// 独有的 commit 数（left-only = ahead，right-only = behind）。
// rev-list 对缺失的 ref 报错退出，此时降级为零值不报错（缺 ref 属业务空值场景）。
func aheadBehindRefs(path, leftRef, rightRef string) (ahead, behind int, err error) {
	if !isGitRepo(path) {
		return 0, 0, nil
	}
	out, err := runOut(path, "rev-list", "--left-right", "--count", leftRef+"..."+rightRef)
	if err != nil {
		return 0, 0, nil
	}
	ahead, behind, ok := parseCountPair(out)
	if !ok {
		return 0, 0, nil
	}
	return ahead, behind, nil
}

// parseCountPair 解析 rev-list --count 的 "ahead\tbehind" 输出。
func parseCountPair(out string) (int, int, bool) {
	left, right, ok := strings.Cut(strings.TrimSpace(out), "\t")
	if !ok {
		return 0, 0, false
	}
	a, errA := strconv.Atoi(left)
	b, errB := strconv.Atoi(right)
	if errA != nil || errB != nil {
		return 0, 0, false
	}
	return a, b, true
}
