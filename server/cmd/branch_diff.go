package cmd

// 本文件收敛「本地分支 × remote 同名分支」差距计算的共享助手，
// 供 info -v 的分支同步状态段与 pull 的候选分支计算复用。
// 口径：基于本地记录的 remote 跟踪分支（refs/remotes/*，不联网），
// 即上次 fetch 时的远端状态。

import (
	"cube/util/git"
	"fmt"
	"sort"
	"strings"
)

// buildRemoteBranchMap 把远程分支列表组织成 map[branch]set[remote]，便于 O(1) 查询
// 「某 remote 是否有某分支」。
func buildRemoteBranchMap(remoteBranches []git.RemoteBranch) map[string]map[string]bool {
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
func buildStatusHeaders(remotes []git.Remote) []string {
	headers := make([]string, 0, 1+len(remotes))
	headers = append(headers, "Branch")
	for _, r := range remotes {
		headers = append(headers, r.Name)
	}
	return headers
}

// branchRemoteDiff 描述一个本地分支相对一个 remote 同名分支的领先/落后 commit 数。
// info -v 的同步宽表与 pull/push 建议都基于它。
type branchRemoteDiff struct {
	Branch string
	Remote string
	Ahead  int // 本地领先
	Behind int // 本地落后（远端有更新）
}

// collectBranchRemoteDiffs 计算每个 (分支 × remote) 对的 ahead/behind，
// 只含该 remote 上存在同名分支的对（无对比对象的格子不产生数据）。
func collectBranchRemoteDiffs(
	repoPath string,
	branches []string,
	remotes []git.Remote,
	remoteHasBranch map[string]map[string]bool,
) []branchRemoteDiff {
	var diffs []branchRemoteDiff
	for _, b := range branches {
		for _, r := range remotes {
			if !remoteHasBranch[b][r.Name] {
				continue
			}
			ahead, behind, _ := git.AheadBehindRemote(repoPath, b, r.Name, b)
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
func buildStatusRowsFromDiffs(diffs []branchRemoteDiff, branches []string, remotes []git.Remote, currentBranch string) [][]string {
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
