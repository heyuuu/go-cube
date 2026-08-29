package project

// 打开目标（提案 1032 + 1030）：project = git 仓库身份，
// 打开目标 = { 根目录, 主项目 workspaces…, 各 worktree 根及其 workspaces… }。
// worktree / workspace 的可见性都来自主项目 projcache 快照的枚举（读路径不跑 git、不读声明文件）。
//
// OpenTargets 是 CLI 专用出口（cube open 的交互选择、alfred 的平铺展开、
// Web project/open 的目录归属校验）；Web 前端不消费它（无对应 HTTP 端点），
// 前端下拉从 project/list DTO 的快照字段自行拼装同一套排序——改排序/展示规则时两边同步。

import (
	"os"
	"path/filepath"

	"cube/project/projcache"
	"cube/project/workspace"
)

// TargetFlags 打开目标的位标记：一个目标可同时命中多类身份——
// worktree 下的 workspace 目录 = FlagWorktree|FlagWorkspace（「长在 worktree 里的
// workspace 子目录」，两类语义都成立）；根目录 = 0。
type TargetFlags uint8

const (
	FlagWorktree  TargetFlags = 1 << iota // linked worktree（根或其下目录）
	FlagWorkspace                         // workspace 成员子目录
)

// OpenTarget 项目的一个打开目标。
type OpenTarget struct {
	Path   string      // 目标目录绝对路径
	Label  string      // 展示名：主目录固定「主目录」；worktree 取分支名，冲突/detached 回退目录名；workspace 取声明/推导名
	Branch string      // worktree 检出分支短名（其余为空）
	Flags  TargetFlags // 身位标记（root=0 / FlagWorktree / FlagWorkspace / 组合）
}

// IsWorktree 目标位于 linked worktree 内（worktree 根或其下）。
func (t OpenTarget) IsWorktree() bool { return t.Flags&FlagWorktree != 0 }

// IsWorkspace 目标是 workspace 成员子目录。
func (t OpenTarget) IsWorkspace() bool { return t.Flags&FlagWorkspace != 0 }

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
	targets = append(targets, workspaceTargetsAt(root, info.Workspaces, 0)...)
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
		targets = append(targets, OpenTarget{Path: wt.Path, Label: label, Branch: wt.Branch, Flags: FlagWorktree})
		targets = append(targets, workspaceTargetsAt(wt.Path, wt.Workspaces, FlagWorktree)...)
	}
	return targets
}

// workspaceTargetsAt 把相对所属根的 workspace 成员展开为绝对路径目标；
// baseFlags 是所属根的标记（主根 = 0，worktree 根 = FlagWorktree），叠加 FlagWorkspace。
func workspaceTargetsAt(root string, ws []workspace.Workspace, baseFlags TargetFlags) []OpenTarget {
	if len(ws) == 0 {
		return nil
	}
	targets := make([]OpenTarget, 0, len(ws))
	for _, w := range ws {
		abs := filepath.Join(root, w.Path)
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		targets = append(targets, OpenTarget{Path: abs, Label: w.Name, Flags: baseFlags | FlagWorkspace})
	}
	return targets
}
