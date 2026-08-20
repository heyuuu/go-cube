package workbench

import (
	"errors"
	"fmt"

	"cube/util/git"
)

// --- git 面板（info / 分支与 tag / commit 日志 / 工作副本快照）---

// Info 工作台项目基本信息：入口目录规范化的仓库根 + 默认分支。
// 工作副本列表归 /worktrees 快照（含状态，刷新节奏不同）。
type Info struct {
	Root          string `json:"root"`          // 仓库根目录（path 向上探测 .git 的结果，主目录与 worktree 进来得到同一结果）
	DefaultBranch string `json:"defaultBranch"` // 远端默认分支（无 remote 时为空，可接受空值）
}

// Refs 分支与 tag 信息
type Refs struct {
	Locals  []string           `json:"locals"`  // 本地分支名
	Current string             `json:"current"` // 当前检出分支（无检出为空）
	Remotes []git.RemoteBranch `json:"remotes"` // 远程分支（所有 remote）
	Tags    []string           `json:"tags"`    // 全部 tag 名
}

// CommitsPageResult commit 日志一页数据（纯列表，泳道布局由前端对已持有数据计算）。
// Cursor 用 skip 偏移（依赖 git log 对同一 ref 集合的确定序），前端按 sha 去重兜底翻页边界。
type CommitsPageResult struct {
	List       []git.CommitEntry `json:"list"`
	NextCursor int               `json:"nextCursor"` // 下一页 skip 偏移；HasMore=false 时无意义
	HasMore    bool              `json:"hasMore"`    // 还有没有下一页（后端多取 1 条探测，整倍边界不误报）
}

// WorktreeStatus 单个工作副本的快照：WorktreeList 的身份字段 + LoadRepoStatus 的状态汇总。
// 是工作副本徽标与 commit 图虚拟节点（前端构造）的共同数据源；bare 副本的状态字段为零值。
type WorktreeStatus struct {
	Path      string `json:"path"`      // 工作副本绝对路径
	Head      string `json:"head"`      // 当前 HEAD commit sha（虚拟节点的挂载点）
	Branch    string `json:"branch"`    // 检出分支短名；detached/bare 为空
	Detached  bool   `json:"detached"`  // HEAD 游离
	Bare      bool   `json:"bare"`      // 裸仓库（无工作区，无未提交概念）
	Dirty     bool   `json:"dirty"`     // 任一变更（staged/unstaged/untracked）
	Ahead     int    `json:"ahead"`     // 领先上游的提交数
	Behind    int    `json:"behind"`    // 落后上游的提交数
	Staged    int    `json:"staged"`    // 暂存区变更文件数
	Unstaged  int    `json:"unstaged"`  // 工作区变更文件数（不含 untracked）
	Untracked int    `json:"untracked"` // 未跟踪文件数
}

// --- 文件树 / 文件读写 / diff（代码阅读面板与 diff 面板）---

// SourceType TreeSource 的类型：commit（历史提交）/ ref（分支或 tag）/ worktree（工作副本当前文件状态）。
// 三类目标在工作台里统一被「选中」与「对比」（见总纲提案 1008 的核心抽象）。
type SourceType string

const (
	SourceTypeCommit   SourceType = "commit"
	SourceTypeRef      SourceType = "ref"
	SourceTypeWorktree SourceType = "worktree"
)

// TreeSource 工作台统一的目标抽象：
//   - commit:   Id = commit sha
//   - ref:      Id = 分支名 / tag 等 ref 名
//   - worktree: Id = 工作副本目录绝对路径（含未提交改动的当前状态）
type TreeSource struct {
	Type SourceType `json:"type"`
	Id   string     `json:"id"`
}

// ParseTreeSource HTTP query 传参用扁平的 sourceType/sourceId（双源场景 leftType/leftId、rightType/rightId），
// ParseTreeSource 负责解析与校验。
func ParseTreeSource(typeStr, id string) (TreeSource, error) {
	switch SourceType(typeStr) {
	case SourceTypeCommit, SourceTypeRef, SourceTypeWorktree:
	default:
		return TreeSource{}, fmt.Errorf("未知的 sourceType: %q（合法值 commit/ref/worktree）", typeStr)
	}
	if id == "" {
		return TreeSource{}, errors.New("sourceId 不能为空")
	}
	return TreeSource{Type: SourceType(typeStr), Id: id}, nil
}

// TreeListResult 全量文件清单：扁平相对路径（前端用 lib/tree 组树）。
// 统一只含 git 管理的文件——worktree 源含未跟踪未忽略项，被忽略项在两种源下都不返回。
type TreeListResult struct {
	List []string `json:"list"`
}
