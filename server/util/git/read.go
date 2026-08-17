package git

// 读操作：全部走系统 git 子进程的机器可读输出（for-each-ref --format /
// status --porcelain / rev-list --count / symbolic-ref / remote -v）。
//
// 为什么读操作也用系统 git（历史上曾用 go-git 库）：
//   - 行为一致：go-git 的 Status 不加载全局忽略链（core.excludesFile / XDG），
//     输出与真实 git 有偏差，需调用方手工补规则；且其忽略过滤发生在全量
//     diff 之后，untracked 文件每次全量读内容算 SHA，大仓库单次可到秒级。
//     原生 git 自动加载完整忽略链、自带 index 缓存，同场景毫秒级；
//   - 开销可接受：早期「CLI 批量采集要 N×3 次 fork」的顾虑已随 gitcache
//     异步采集消失——读路径只读快照，采集在后台批量进行。
//
// 输出稳定性（规避用户本地 git 配置差异，见 runOut 与各命令的显式 flag）：
//   - 只解析机器可读格式，不解析任何面向人的文案（stderr 会随 locale 翻译）；
//   - LC_ALL=C、-c core.quotePath=false、GIT_PAGER=cat 统一注入；
//   - status 显式带 --untracked-files=normal，覆盖 status.showUntrackedFiles=no。
//
// 错误处理约定（全包一致，上层缓存层依赖）：
//   - 非 git 目录、缺失 remote/分支等「业务上可接受的空值」场景：返回零值 + nil；
//   - 真实读取错误（损坏的 .git、IO 异常等）：返回零值 + error，由调用方决定是否记录。

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// readCmdLogEnabled 控制「git 读命令」逐条日志的开关。
// gitcache 全量采集时一次会跑几百条 git 子进程，逐条 Debug 日志会淹没其他输出；
// 默认静默，仅设置 CUBE_GITCACHE_TRACE（任意非空值）时打印，级别保持 Debug。
var readCmdLogEnabled = sync.OnceValue(func() bool {
	return os.Getenv("CUBE_GITCACHE_TRACE") != ""
})

// runOut 在 dir 下执行 git 读命令并捕获 stdout（不透传终端）。
// 与 Run 的差异：输出面向程序解析而非人，因此注入与用户配置无关的稳定环境：
//   - LC_ALL=C：统一 locale（porcelain 格式本身不受 locale 影响，防御性兜底）；
//   - GIT_PAGER=cat：禁用分页器，避免极端配置下进程等待交互翻页挂起；
//   - --no-optional-locks：后台读不碰 index.lock，不与用户正在进行的 git 操作抢锁；
//   - -c core.quotePath=false：非 ASCII 路径（如中文文件名）不转义成八进制串。
//
// stderr 不做解析（文案随 locale 翻译，不可依赖），只作为错误信息附带给日志。
func runOut(dir string, args ...string) (string, error) {
	args = append([]string{"--no-optional-locks", "-c", "core.quotePath=false"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if readCmdLogEnabled() {
		slog.Debug("git 读命令", "cmd", cmd.String())
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s 执行失败: %w；stderr: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// isGitRepo 判断 path 自身是否为 git 仓库根（存在 .git 文件或目录，worktree 的
// .git 文件也算）。bare 仓库（目录本身即 gitdir，无 .git）按非仓库降级——
// cube 收录的项目必然是普通工作区副本。
//
// 读命令靠它把「非仓库」从子进程错误里区分出来：git 的报错文案随 locale 变化，
// 不能解析 stderr 识别，而一次 os.Stat 的预判开销可以忽略。
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// RemoteUrl 返回 path 处仓库 origin remote 的 URL。
// 无 origin remote 时返回 ("", nil)，不视为错误。
func RemoteUrl(path string) (string, error) {
	if !isGitRepo(path) {
		return "", nil // 非仓库目录：返回空值，不报错
	}
	out, err := runOut(path, "remote", "get-url", "origin")
	if err != nil {
		return "", nil // 无 origin remote：返回空值，不报错
	}
	// remote 可配置多个 URL（每行一个），取第一条
	return firstLine(out), nil
}

// firstLine 取输出第一行并去首尾空白，空输出返回空串。
func firstLine(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	return strings.TrimSpace(line)
}

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

// AheadBehind 计算本地分支 local 相对远程分支 remote（"origin/master" 形式）的
// 领先 / 落后 commit 数。
//   - ahead  = 本地有、远程没有的 commit 数（待推送）
//   - behind = 远程有、本地没有的 commit 数（待拉取）
//
// 任一 ref 缺失返回 (0, 0, nil)。基于本地已有 commit 比对（不 fetch），
// 未 fetch 过的数据可能不准——与「缓存场景接受 stale」的整体策略一致。
func AheadBehind(path string, local, remote string) (ahead, behind int, err error) {
	return aheadBehindRefs(path, "refs/heads/"+local, "refs/remotes/"+remote)
}

// AheadBehindRemote 计算本地分支 localBranch 相对指定 remote 的 remoteBranch 的
// 领先 / 落后数。与 AheadBehind 的区别：显式接受 remote 名，便于比较非 origin 的远程分支。
// 任一 ref 缺失（如该 remote 没有这个分支）返回 (0, 0, nil)，不视为错误。
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

// IsDirty 返回 path 处仓库的工作区是否有改动（含 untracked，尊重仓库内
// .gitignore 与全局忽略规则——原生 git 自动加载完整忽略链）。
// 非仓库目录返回 (false, nil)。
func IsDirty(path string) (bool, error) {
	if !isGitRepo(path) {
		return false, nil
	}
	out, err := runOut(path, statusPorcelainArgs()...)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

// FileStatus 单个文件的工作区状态，展示形态对齐 git status --short。
type FileStatus struct {
	Code string // 两字符状态码 XY（X=暂存区，Y=工作区），如 "M " / " M" / "??" / "A " / "D " / "R "
	Path string // 相对仓库根路径；已暂存的 rename/copy 为 "旧路径 -> 新路径"
}

// StatusFiles 返回 path 处仓库工作区有变动的文件列表（含 untracked，尊重仓库内
// .gitignore 与全局忽略规则），按路径排序（git 输出本身有序）。
// 非仓库目录或工作区干净时返回 (nil, nil)，不视为错误。
func StatusFiles(path string) ([]FileStatus, error) {
	if !isGitRepo(path) {
		return nil, nil
	}
	out, err := runOut(path, append(statusPorcelainArgs(), "-z")...)
	if err != nil {
		return nil, err
	}
	return parseStatusPorcelain(out), nil
}

// statusPorcelainArgs 是 status 读命令的统一参数：
//   - --untracked-files=normal 显式覆盖用户 status.showUntrackedFiles=no 的配置
//     （该配置会静默吞掉 untracked 文件，导致 dirty 误判）；normal 档不深入
//     未跟踪目录内部，够用且快。
func statusPorcelainArgs() []string {
	return []string{"status", "--porcelain", "--untracked-files=normal"}
}

// parseStatusPorcelain 解析 `git status --porcelain -z` 输出：每条记录 "XY 路径"，
// 以 NUL 分隔（路径含引号/反斜杠等特殊字符时无需反转义）；暂存区 rename/copy
// （X 为 R/C）的记录后跟第二条原路径，展示时合成 "旧 -> 新"。
func parseStatusPorcelain(out string) []FileStatus {
	parts := strings.Split(out, "\x00")
	var files []FileStatus
	for i := 0; i < len(parts); i++ {
		rec := parts[i]
		if len(rec) < 4 { // 至少 "XY " + 一个字符的路径
			continue
		}
		code, path := rec[:2], rec[3:]
		if rec[0] == 'R' || rec[0] == 'C' {
			if i+1 < len(parts) && parts[i+1] != "" {
				path = parts[i+1] + " -> " + path
				i++
			}
		}
		files = append(files, FileStatus{Code: code, Path: path})
	}
	return files
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
