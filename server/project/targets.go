package project

// 打开目标（提案 1032 + 1030）：project = git 仓库身份，
// 打开目标 = { 根目录, 主项目 workspaces…, 各 worktree 根及其 workspaces… }。
// worktree / workspace 的可见性都来自主项目 projcache 快照的枚举（读路径不跑 git、不读声明文件）。

import (
	"os"
	"path/filepath"

	"cube/project/projcache"
	"cube/project/workspace"
)

// TargetKind 打开目标的身份类型（1030）：描述「目录以什么身份成为打开目标」，
// 不编码归属关系——归属由排序分组的天然结构表达（workspace 跟随其所属根）。
type TargetKind string

const (
	KindRoot      TargetKind = "root"      // 主项目根目录
	KindWorktree  TargetKind = "worktree"  // linked worktree 根目录
	KindWorkspace TargetKind = "workspace" // workspace 成员子目录（不管长在主根还是 worktree 根下）
)

// OpenTarget 项目的一个打开目标。
type OpenTarget struct {
	Path   string     // 目标目录绝对路径
	Label  string     // 展示名：根目录固定「根目录」；worktree 取分支名，冲突/detached 回退目录名；workspace 取声明/推导名
	Branch string     // worktree 检出分支短名（其余为空）
	Kind   TargetKind // 目标身份（root / worktree / workspace）
}

// targetEntries 由项目根路径 + 快照条目构造全部非根目标（纯函数，便于单测）。
// 排序（1030 定稿）：主项目 workspaces → 每个 worktree 根 → 该 worktree 的 workspaces。
// worktree 展示名规则（提案 1032）：取分支名；分支为空（detached）或同分支被多个
// worktree 检出时回退目录名。workspaces 已在采集侧做过存在性校验，这里再兜一层
// os.Stat（同 worktree 的过滤口径：目录在两次采集之间被删时快照仍残留，目标必须真实可打开）。
func targetEntries(root string, info *projcache.Entry) []OpenTarget {
	if info == nil {
		return nil
	}
	targets := make([]OpenTarget, 0, len(info.Workspaces)+2*len(info.Worktrees))
	targets = append(targets, workspaceTargetsAt(root, info.Workspaces)...)
	branchCount := make(map[string]int, len(info.Worktrees))
	for _, wt := range info.Worktrees {
		if wt.Branch != "" {
			branchCount[wt.Branch]++
		}
	}
	for _, wt := range info.Worktrees {
		label := wt.Branch
		if label == "" || branchCount[label] > 1 {
			label = filepath.Base(wt.Path)
		}
		targets = append(targets, OpenTarget{Path: wt.Path, Label: label, Branch: wt.Branch, Kind: KindWorktree})
		targets = append(targets, workspaceTargetsAt(wt.Path, wt.Workspaces)...)
	}
	return targets
}

// workspaceTargetsAt 把相对所属根的 workspace 成员展开为绝对路径目标。
func workspaceTargetsAt(root string, ws []workspace.Workspace) []OpenTarget {
	if len(ws) == 0 {
		return nil
	}
	targets := make([]OpenTarget, 0, len(ws))
	for _, w := range ws {
		abs := filepath.Join(root, w.Path)
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		targets = append(targets, OpenTarget{Path: abs, Label: w.Name, Kind: KindWorkspace})
	}
	return targets
}
