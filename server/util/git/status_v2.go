package git

import (
	"strconv"
	"strings"
)

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
	out, err := runOut(dir, append(statusPorcelainArgs(), "--porcelain=v2", "--branch")...)
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
