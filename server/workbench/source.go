package workbench

import (
	"errors"
	"fmt"
)

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

// HTTP query 传参用扁平的 sourceType/sourceId（双源场景 leftType/leftId、rightType/rightId），
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
