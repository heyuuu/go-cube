package git

// 工作区状态查询，同一 status 命令的三个视角：
//   - StatusFiles：porcelain v1，逐文件状态码明细；
//   - LoadRepoStatus：porcelain v2 --branch，一次拿分支/sha/上游/ahead-behind/计数汇总
//     （dirty 布尔即其 Dirty 字段，无单独的 IsDirty）；
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

// statusPorcelainArgs 是 porcelain v1 读命令（StatusFiles / LoadIgnored）的公共参数：
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

// RepoStatus 工作副本状态汇总（提案 1011）：一次 `status --porcelain=v2 --branch`
// 拿全分支/上游/ahead-behind/变更计数，避免多条命令拼装。
type RepoStatus struct {
	Branch    string `json:"branch"`    // 检出分支名，detached 为空
	Sha       string `json:"sha"`       // 当前 HEAD sha
	Detached  bool   `json:"detached"`  // HEAD 游离
	Upstream  string `json:"upstream"`  // 上游分支（如 origin/main），无则为空
	Ahead     int    `json:"ahead"`     // 领先上游的提交数
	Behind    int    `json:"behind"`    // 落后上游的提交数
	Staged    int    `json:"staged"`    // 暂存区变更文件数
	Unstaged  int    `json:"unstaged"`  // 工作区变更文件数（不含 untracked）
	Untracked int    `json:"untracked"` // 未跟踪文件数
	Dirty     bool   `json:"dirty"`     // 任一变更（staged/unstaged/untracked）即为 true
}

// LoadRepoStatus 返回 dir 工作副本的状态汇总。非 git 目录返回零值 + nil（与包内
// StatusFiles 等读函数的「可接受空值」约定一致），调用方需先确认目录有效。
func LoadRepoStatus(dir string) (*RepoStatus, error) {
	if !isGitRepo(dir) {
		return &RepoStatus{}, nil
	}
	// v2 汇总不共用 statusPorcelainArgs：porcelain 与 porcelain=v2 是同一选项的
	// 两档，混传靠「后者覆盖前者」侥幸工作；显式只传 v2 一档。
	out, err := runOut(dir, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
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
		case strings.HasPrefix(line, "1 "), strings.HasPrefix(line, "2 "), strings.HasPrefix(line, "u "):
			fields := strings.Fields(line)
			xy := fields[1]
			if xy[0] != '.' && xy[0] != ' ' {
				st.Staged++
			}
			if len(xy) > 1 && xy[1] != '.' && xy[1] != ' ' {
				st.Unstaged++
			}
		}
	}
	st.Dirty = st.Staged > 0 || st.Unstaged > 0 || st.Untracked > 0
	return st
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
// 一致：.gitignore + 全局忽略链），subDir 限定收集范围（相对仓库根，空 = 全仓），
// 返回的路径仍相对仓库根。非仓库目录返回空集合，不视为错误。
//
// 用 status --ignored 而非 ls-files --others --ignored --directory：
// 后者会把「整个目录未跟踪」的目录折叠成一项，混合目录（部分忽略部分不忽略）
// 内的忽略文件随之从列表消失；status 只折叠全忽略目录，混合目录会下钻。
func LoadIgnored(path string, subDir string) (*Ignored, error) {
	ig := &Ignored{Dirs: map[string]bool{}, Files: map[string]bool{}}
	if !isGitRepo(path) {
		return ig, nil
	}
	args := append(statusPorcelainArgs(), "--ignored", "-z")
	if subDir != "" {
		args = append(args, "--", subDir)
	}
	out, err := runOut(path, args...)
	if err != nil {
		return nil, err
	}
	// 只关心 "!!" 记录（忽略项无 rename 形态，路径单段）；目录项带尾部 /
	for _, rec := range strings.Split(out, "\x00") {
		if len(rec) < 4 || rec[:3] != "!! " {
			continue
		}
		if p := rec[3:]; strings.HasSuffix(p, "/") {
			ig.Dirs[strings.TrimSuffix(p, "/")] = true
		} else {
			ig.Files[p] = true
		}
	}
	return ig, nil
}
