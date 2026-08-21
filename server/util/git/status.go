package git

// 工作区状态查询，同一 status 命令的两个视角：
//   - LoadRepoStatus：porcelain v2 --branch，一次拿分支/sha/上游/ahead-behind/计数
//     汇总与逐文件明细（Files）（dirty 布尔即其 Dirty 字段，无单独的 IsDirty）；
//   - LoadIgnored：--ignored，被忽略路径的集合。
// 执行核与错误约定见 run.go。

import (
	"strconv"
	"strings"
)

// FileStatus 单个文件的工作区状态，展示形态对齐 git status --short。
type FileStatus struct {
	Code string // 两字符状态码 XY（X=暂存区，Y=工作区），如 "M " / " M" / "??" / "A " / "D " / "R "
	Path string // 相对仓库根路径；已暂存的 rename/copy 为 "旧路径 -> 新路径"
}

// statusPorcelainArgs 是两个 status 读函数（LoadRepoStatus / LoadIgnored）的公共参数：
//   - --untracked-files=normal 显式覆盖用户 status.showUntrackedFiles=no 的配置
//     （该配置会静默吞掉 untracked 文件，导致 dirty 误判）；normal 档不深入
//     未跟踪目录内部，够用且快。
func statusPorcelainArgs() []string {
	return []string{"status", "--porcelain=v2", "--untracked-files=normal"}
}

// RepoStatus 工作副本状态汇总（提案 1011）：一次 `status --porcelain=v2 --branch`
// 拿全分支/上游/ahead-behind/变更计数与逐文件明细，避免多条命令拼装。
type RepoStatus struct {
	Branch    string       `json:"branch"`    // 检出分支名，detached 为空
	Sha       string       `json:"sha"`       // 当前 HEAD sha
	Detached  bool         `json:"detached"`  // HEAD 游离
	Upstream  string       `json:"upstream"`  // 上游分支（如 origin/main），无则为空
	Ahead     int          `json:"ahead"`     // 领先上游的提交数
	Behind    int          `json:"behind"`    // 落后上游的提交数
	Staged    int          `json:"staged"`    // 暂存区变更文件数
	Unstaged  int          `json:"unstaged"`  // 工作区变更文件数（不含 untracked）
	Untracked int          `json:"untracked"` // 未跟踪文件数
	Dirty     bool         `json:"dirty"`     // 任一变更（staged/unstaged/untracked）即为 true
	Files     []FileStatus `json:"files"`     // 逐文件明细，按 git 输出顺序（路径升序）
}

// LoadRepoStatus 返回 dir 工作副本的状态汇总（含逐文件明细 Files）。非 git 目录
// 返回零值 + nil（包内读函数的「可接受空值」约定一致），调用方需先确认目录有效。
func LoadRepoStatus(dir string) (*RepoStatus, error) {
	if !isGitRepo(dir) {
		return &RepoStatus{}, nil
	}
	out, err := runOut(dir, append(statusPorcelainArgs(), "--branch")...)
	if err != nil {
		return nil, err
	}
	return parseStatusV2(out), nil
}

// parseStatusV2 解析 porcelain v2 + --branch 输出。行首标记：
//
//	# branch.head <name>|detached   # branch.oid <sha>
//	# branch.up <upstream>           # branch.ab +N -M
//	1/2 <XY…>（已跟踪变更）、u（合并冲突）、?（untracked）
//
// 文件行的路径在行尾（前面是固定数量的字段），不能用 Fields 取——路径可含空格；
// rename/copy 的 `2` 行多一个 R<score> 字段，随后是 "新路径\t旧路径"。
func parseStatusV2(out string) *RepoStatus {
	st := &RepoStatus{}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			v := strings.TrimPrefix(line, "# branch.head ")
			if v == "detached" {
				st.Detached = true
			} else {
				st.Branch = v
			}
		case strings.HasPrefix(line, "# branch.oid "):
			st.Sha = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.up "):
			st.Upstream = strings.TrimPrefix(line, "# branch.up ")
		case strings.HasPrefix(line, "# branch.ab "):
			for _, f := range strings.Fields(strings.TrimPrefix(line, "# branch.ab")) {
				if n, err := strconv.Atoi(f[1:]); err == nil {
					if f[0] == '+' {
						st.Ahead = n
					} else if f[0] == '-' {
						st.Behind = n
					}
				}
			}
		case strings.HasPrefix(line, "? "):
			st.Untracked++
			if p := strings.TrimPrefix(line, "? "); p != "" {
				st.Files = append(st.Files, FileStatus{Code: "??", Path: p})
			}
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "u "):
			// `1`：1 XY sub mH mI mW hH hI 路径；`u` 多一对三方 hash（11 段）
			n := 9
			if line[0] == 'u' {
				n = 11
			}
			parts := strings.SplitN(line, " ", n)
			if len(parts) < n {
				continue
			}
			xy, path := parts[1], parts[n-1]
			st.countXY(xy)
			if path != "" {
				st.Files = append(st.Files, FileStatus{Code: xyShort(xy), Path: path})
			}
		case strings.HasPrefix(line, "2 "):
			// 2 XY sub mH mI mW hH hI R<score> 新路径\t旧路径
			parts := strings.SplitN(line, " ", 10)
			if len(parts) < 10 {
				continue
			}
			xy, tail := parts[1], parts[9]
			st.countXY(xy)
			if new, old, ok := strings.Cut(tail, "\t"); ok && new != "" && old != "" {
				st.Files = append(st.Files, FileStatus{Code: xyShort(xy), Path: old + " -> " + new})
			}
		}
	}
	st.Dirty = st.Staged > 0 || st.Unstaged > 0 || st.Untracked > 0
	return st
}

// countXY 按 XY 状态码累计 Staged / Unstaged 计数。
func (st *RepoStatus) countXY(xy string) {
	if xy[0] != '.' && xy[0] != ' ' {
		st.Staged++
	}
	if len(xy) > 1 && xy[1] != '.' && xy[1] != ' ' {
		st.Unstaged++
	}
}

// xyShort 把 v2 的 XY 码（未变更列为 '.'）转回 porcelain v1 / --short 的展示形态
// （未变更列为空格），如 "M." → "M "、".M" → " M"。
func xyShort(xy string) string {
	b := []byte(xy)
	for i, c := range b {
		if c == '.' {
			b[i] = ' '
		}
	}
	return string(b)
}

// Ignored 工作副本中被 git 忽略的路径集合（相对仓库根）。
// 目录与文件分开存：目录级命中即其下全部内容忽略。
type Ignored struct {
	Dirs  map[string]bool // 被忽略的目录（不含尾部 /）
	Files map[string]bool // 被忽略的文件
}

// Has 判断单个路径是否在忽略集合中（不含「位于忽略目录之下」的推断，那是调用方的遍历语义）。
func (ig *Ignored) Has(path string) bool { return ig.Files[path] || ig.Dirs[path] }

// LoadIgnored 加载 path 工作副本中被 git 忽略的路径（口径与 git status --ignored
// 一致：.gitignore + 全局忽略链），路径相对仓库根。非仓库目录返回空集合，不视为错误。
//
// 用 status --ignored 而非 ls-files --others --ignored --directory：
// 后者会把「整个目录未跟踪」的目录折叠成一项，混合目录（部分忽略部分不忽略）
// 内的忽略文件随之从列表消失；status 只折叠全忽略目录，混合目录会下钻。
func LoadIgnored(path string) (*Ignored, error) {
	ig := &Ignored{Dirs: map[string]bool{}, Files: map[string]bool{}}
	if !isGitRepo(path) {
		return ig, nil
	}
	out, err := runOut(path, append(statusPorcelainArgs(), "--ignored")...)
	if err != nil {
		return nil, err
	}
	// 只关心 "! 路径" 记录（忽略项无 rename 形态，路径即行尾）；目录项带尾部 /
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "! ") {
			continue
		}
		if p := strings.TrimPrefix(line, "! "); strings.HasSuffix(p, "/") {
			ig.Dirs[strings.TrimSuffix(p, "/")] = true
		} else {
			ig.Files[p] = true
		}
	}
	return ig, nil
}
