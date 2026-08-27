package project

// 打开目标（提案 1032）：project = git 仓库身份，打开目标 = { 根目录, worktrees… }。
// worktree 不再是独立项目，其可见性来自主项目 gitcache 快照的枚举（不现场跑 git）。

import (
	"path/filepath"

	"cube/project/gitcache"
)

// OpenTarget 项目的一个打开目标：根目录或 linked worktree（1030 落地后加入 workspace）。
type OpenTarget struct {
	Path   string // 目标目录绝对路径
	Label  string // 展示名：根目录固定「根目录」；worktree 取分支名，冲突/detached 回退目录名
	Branch string // worktree 检出分支短名（根目录为空）
}

// worktreeTargets 由快照条目构造 worktree 目标列表（纯函数，便于单测）。
// 展示名规则（提案 1032）：取分支名；分支为空（detached）或同分支被多个 worktree 检出时回退目录名。
func worktreeTargets(info *gitcache.Entry) []OpenTarget {
	if info == nil {
		return nil
	}
	branchCount := make(map[string]int, len(info.Worktrees))
	for _, wt := range info.Worktrees {
		if wt.Branch != "" {
			branchCount[wt.Branch]++
		}
	}
	targets := make([]OpenTarget, 0, len(info.Worktrees))
	for _, wt := range info.Worktrees {
		label := wt.Branch
		if label == "" || branchCount[label] > 1 {
			label = filepath.Base(wt.Path)
		}
		targets = append(targets, OpenTarget{Path: wt.Path, Label: label, Branch: wt.Branch})
	}
	return targets
}
