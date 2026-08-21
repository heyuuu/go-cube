package git

// ref 查询：本地与远程分支 / tag / 默认分支 / HEAD 与父提交 / ahead-behind。
// remote 配置查询（remote -v）在 remote.go。执行核与错误约定见 run.go。

import (
	"errors"
	"strconv"
	"strings"
)

// ref namespace 前缀：git 规范全名的三棵主子树（本包多处与 workbench/cmd 的
// 拼装/前缀判定共用；refs/ 树本身开放，不止这三棵）
const (
	RefHeadsPrefix   = "refs/heads/"
	RefRemotesPrefix = "refs/remotes/"
	RefTagsPrefix    = "refs/tags/"
)

type Ref struct {
	Name      string `json:"name"`      // Ref Name，refs/{heads,remotes,tags}/xxx
	ShortName string `json:"shortName"` // Short Name，省略 refs/xxx/ 前缀；注意 remote 的 ShortName 含 remote 名
	Remote    string `json:"remote"`    // Remote，仅 refs/remotes/* 有效，远端 remote 名 (例如 origin)
	Branch    string `json:"branch"`    // 分支名，在 refs/heads/* 及 refs/remotes/* 时有效，分支名 (例如 master)
}

func (r Ref) IsLocal() bool {
	return r.Remote == "" && r.Branch != ""
}

func (r Ref) IsRemote() bool {
	return r.Remote != "" && r.Branch != ""
}

func (r Ref) IsTag() bool {
	return r.Branch == ""
}

// BuildRef 构建 Ref 实例
func BuildRef(refName string) (Ref, error) {
	refName = strings.TrimSpace(refName)
	switch {
	case strings.HasPrefix(refName, RefHeadsPrefix):
		shortName := strings.TrimPrefix(refName, RefHeadsPrefix)
		return Ref{Name: refName, ShortName: shortName, Branch: shortName}, nil
	case strings.HasPrefix(refName, RefTagsPrefix):
		shortName := strings.TrimPrefix(refName, RefTagsPrefix)
		return Ref{Name: refName, ShortName: shortName}, nil
	case strings.HasPrefix(refName, RefRemotesPrefix):
		shortName := strings.TrimPrefix(refName, RefRemotesPrefix)

		// 拆分 remote 名 和 branch 名
		idx := strings.Index(shortName, "/")
		if idx <= 0 {
			break
		}
		remote, branch := shortName[:idx], shortName[idx+1:]

		// refs/remotes/<remote>/HEAD 是远端默认分支的符号指针而非真实分支，跳过
		if branch == "HEAD" {
			break
		}

		return Ref{
			Name:      refName,
			ShortName: shortName,
			Remote:    remote,
			Branch:    branch,
		}, nil
	}

	// 没匹配上的情况
	return Ref{}, errors.New("暂不支持的 ref 值: " + refName)
}

// RefsResult 全量 ref 清单（规范全名，按 namespace 分组）。
type RefsResult struct {
	Locals  []Ref `json:"locals"`
	Remotes []Ref `json:"remotes"`
	Tags    []Ref `json:"tags"`
}

// BuildRefs 构建 Refs 结果，暴露以供构造测试数据
func BuildRefs(refNames []string) *RefsResult {
	result := &RefsResult{}
	for _, refName := range refNames {
		ref, err := BuildRef(refName)
		if err != nil {
			continue
		}
		if ref.IsLocal() {
			result.Locals = append(result.Locals, ref)
		} else if ref.IsRemote() {
			result.Remotes = append(result.Remotes, ref)
		} else if ref.IsTag() {
			result.Tags = append(result.Tags, ref)
		}
	}
	return result
}

// Refs 一次 for-each-ref 拉全 refs/ 树，按 namespace 前缀分组返回规范全名。
// 只回答「仓库有哪些 ref」；「当前检出在哪个 ref」是 HEAD 状态，用 HeadRef。
// 规范全名的拼装收敛在此（workbench 的 TreeSource ref id 直接透传，不在调用方拼前缀）；
// refs/stash 等其他子树不分组不返回（现有消费方只消费三类）。
// 非仓库返回零值，不视为错误。
func Refs(path string) (*RefsResult, error) {
	if !isGitRepo(path) {
		return &RefsResult{}, nil
	}

	out, err := runOut(path, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return nil, err
	}

	result := BuildRefs(strings.Split(strings.TrimSpace(out), "\n"))
	return result, nil
}

// HeadRef 返回 HEAD 指向的 ref 全名。正常为 refs/heads/*；git 只强制 HEAD 在
// refs/ 内，病态挂载（symbolic-ref 指向 tag 等）时原样返回、不做 heads 假设。
// detached HEAD、非仓库返回空串——这是业务空值而非错误（Branches 的 current
// 短名与 workbench 的当前分支全名都源于此，只差在是否剥前缀）。
func HeadRef(path string) string {
	if !isGitRepo(path) {
		return ""
	}
	// symbolic-ref 失败（非 0 退出）= detached HEAD，降级空串
	out, err := runOut(path, "symbolic-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// CurrentBranch 当前检出分支短名（HEAD 目标 ref 剥 refs/heads/ 前缀）。
// detached、病态挂载到 heads 外、非仓库均返回空——与 git branch --show-current
// 口径一致（git 只强制 HEAD 在 refs/ 内，不强制在 heads 下）。
func CurrentBranch(path string) string {
	ref := HeadRef(path)
	if strings.HasPrefix(ref, RefHeadsPrefix) {
		return strings.TrimPrefix(ref, RefHeadsPrefix)
	}
	return ""
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
	// origin/HEAD 未设置时 symbolic-ref 报错（非 0 退出），走 fallback。
	// 不带 --short 取全名（形如 refs/remotes/origin/master），BuildRef 才能拆出分支名
	if out, err := runOut(path, "symbolic-ref", RefRemotesPrefix+"origin/HEAD"); err == nil {
		if ref, err := BuildRef(strings.TrimSpace(out)); err == nil {
			return ref.Branch, nil
		}
	}
	for _, candidate := range []string{"master", "main"} {
		// show-ref --verify --quiet 只用退出码表达存在性（0=存在）
		if _, err := runOut(path, "show-ref", "--verify", "--quiet", RefHeadsPrefix+candidate); err == nil {
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
	return aheadBehindRefs(path, RefHeadsPrefix+localBranch, RefRemotesPrefix+remoteName+"/"+remoteBranch)
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
