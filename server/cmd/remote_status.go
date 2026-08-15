package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/gogit"
	"cube/util/tui"
)

// newRemoteStatusCmd 是 `cube remote-status` 命令入口。
//
// query 定位已收录项目（规则同 info），以项目根为 git 仓库。
// 列出「本地分支 ∩ 各 remote 同名分支的并集」中，每个分支相对每个 remote 上
// 对应分支的 ahead / behind commit 数。宽表：每 remote 占一列，内容形如 "+3/-1"。
// 当前分支用 "*" 标记；某 remote 没有该分支则该格显示 "-"。
func newRemoteStatusCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote-status [query]",
		Short: "列出本地分支与各 remote 对应分支的 commit 差距（ahead/behind）",
		Long: `以宽表展示每个本地分支相对各 remote 同名分支的 commit 差距，
每个 remote 占一列，内容形如 "+3/-1"。

只统计本地与任一 remote 同名的分支；当前分支名前带 "*"，
某 remote 没有该分支显示 "-"，已同步（0/0）显示 "✓"。

query 定位目标项目（支持项目名或路径模糊搜索，规则同 info 命令），
不传时交互选择；以项目根目录作为 git 仓库。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. 定位项目：query 匹配（规则同 info），以项目根为仓库
			query := getArg(args, 0)
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}
			repoPath := proj.Path()

			// 2. 收集数据：本地分支 / 当前分支 / remote 列表 / 远程分支
			localBranches, currentBranch, err := gogit.Branches(repoPath)
			if err != nil {
				return fmt.Errorf("读取本地分支失败: %w", err)
			}
			remotes, err := gogit.Remotes(repoPath)
			if err != nil {
				return fmt.Errorf("读取 remote 列表失败: %w", err)
			}
			if len(remotes) == 0 {
				return fmt.Errorf("仓库未配置任何 remote: %s", repoPath)
			}
			remoteBranches, err := gogit.RemoteBranches(repoPath)
			if err != nil {
				return fmt.Errorf("读取远程分支失败: %w", err)
			}

			// 3. 算分支集合：本地分支 ∩ {任一 remote 上有同名分支}
			//    remoteHasBranch[branch][remote] = 该 remote 是否有此分支
			remoteHasBranch := buildRemoteBranchMap(remoteBranches)
			branches := pickSharedBranches(localBranches, remoteHasBranch)
			if len(branches) == 0 {
				fmt.Println("没有本地与任一 remote 同名的分支，无可对比项。")
				return nil
			}

			// 4. 计算每格的 ahead/behind，打印宽表
			diffs := collectBranchRemoteDiffs(repoPath, branches, remotes, remoteHasBranch)
			headers := buildStatusHeaders(remotes)
			rows := buildStatusRowsFromDiffs(diffs, branches, remotes, currentBranch)
			tui.PrintTable(headers, rows)
			return nil
		},
	}
	return cmd
}

// buildRemoteBranchMap 把远程分支列表组织成 map[branch]set[remote]，便于 O(1) 查询
// 「某 remote 是否有某分支」。
func buildRemoteBranchMap(remoteBranches []gogit.RemoteBranch) map[string]map[string]bool {
	m := make(map[string]map[string]bool)
	for _, rb := range remoteBranches {
		if m[rb.Branch] == nil {
			m[rb.Branch] = make(map[string]bool)
		}
		m[rb.Branch][rb.Remote] = true
	}
	return m
}

// pickSharedBranches 取「本地分支 ∩ 任一 remote 同名分支」的并集，结果按名排序。
// 本地有但所有 remote 都没有的分支被排除（无对比对象）。
func pickSharedBranches(localBranches []string, remoteHasBranch map[string]map[string]bool) []string {
	var shared []string
	for _, b := range localBranches {
		if len(remoteHasBranch[b]) > 0 {
			shared = append(shared, b)
		}
	}
	sort.Strings(shared)
	return shared
}

// buildStatusHeaders 构造宽表表头：Branch + 每个 remote 名。
func buildStatusHeaders(remotes []gogit.Remote) []string {
	headers := make([]string, 0, 1+len(remotes))
	headers = append(headers, "Branch")
	for _, r := range remotes {
		headers = append(headers, r.Name)
	}
	return headers
}

// branchRemoteDiff 描述一个本地分支相对一个 remote 同名分支的领先/落后 commit 数。
// remote-status 宽表与 info -v 的同步建议都基于它。
type branchRemoteDiff struct {
	Branch string
	Remote string
	Ahead  int // 本地领先
	Behind int // 本地落后（远端有更新）
}

// collectBranchRemoteDiffs 计算每个 (分支 × remote) 对的 ahead/behind，
// 只含该 remote 上存在同名分支的对（无对比对象的格子不产生数据）。
// 数据基于本地记录的 remote 跟踪分支（不联网），即上次 fetch 时的远端状态。
func collectBranchRemoteDiffs(
	repoPath string,
	branches []string,
	remotes []gogit.Remote,
	remoteHasBranch map[string]map[string]bool,
) []branchRemoteDiff {
	var diffs []branchRemoteDiff
	for _, b := range branches {
		for _, r := range remotes {
			if !remoteHasBranch[b][r.Name] {
				continue
			}
			ahead, behind, _ := gogit.AheadBehindRemote(repoPath, b, r.Name, b)
			diffs = append(diffs, branchRemoteDiff{Branch: b, Remote: r.Name, Ahead: ahead, Behind: behind})
		}
	}
	return diffs
}

// buildStatusRowsFromDiffs 由预计算的 (分支×remote) 差距构造宽表行：
//   - 某格的 remote 没有该分支 → "-"（无对比对象，diffs 中也无该对）
//   - ahead/behind 全 0（已同步）→ "✓"
//   - 否则 → "+ahead/-behind"（任一项为 0 时省略，如 "+3" / "-1"）
//
// 当前分支名前加 "*"。
func buildStatusRowsFromDiffs(diffs []branchRemoteDiff, branches []string, remotes []gogit.Remote, currentBranch string) [][]string {
	// (branch, remote) → diff 索引，避免在表格构造里重复算 ahead/behind
	diffAt := make(map[string]branchRemoteDiff, len(diffs))
	for _, d := range diffs {
		diffAt[d.Branch+"@"+d.Remote] = d
	}

	rows := make([][]string, len(branches))
	for i, branch := range branches {
		// 分支名列：当前分支加 *
		name := branch
		if branch == currentBranch {
			name = "* " + branch
		}
		row := make([]string, 1+len(remotes))
		row[0] = name

		for j, r := range remotes {
			if d, ok := diffAt[branch+"@"+r.Name]; ok {
				row[1+j] = formatAheadBehind(d.Ahead, d.Behind)
			} else {
				row[1+j] = "-"
			}
		}
		rows[i] = row
	}
	return rows
}

// formatAheadBehind 把 (ahead, behind) 格式化为一格内容：
//   - 全 0 → "✓"
//   - 否则形如 "+3/-1"；ahead 或 behind 为 0 时省略该半边（"+3" / "-1"）
func formatAheadBehind(ahead, behind int) string {
	if ahead == 0 && behind == 0 {
		return "✓"
	}
	var parts []string
	if ahead > 0 {
		parts = append(parts, fmt.Sprintf("+%d", ahead))
	}
	if behind > 0 {
		parts = append(parts, fmt.Sprintf("-%d", behind))
	}
	return strings.Join(parts, "/")
}
