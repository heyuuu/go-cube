package workbench

import (
	"fmt"

	"cube/util/git"
)

// Info 工作台项目基本信息：入口目录规范化的仓库根 + 全部工作副本 + 默认分支。
type Info struct {
	Root          string         `json:"root"`          // 仓库根目录（path 向上探测 .git 的结果，主目录与 worktree 进来得到同一结果）
	Worktrees     []git.Worktree `json:"worktrees"`     // 全部工作副本（主目录在前），worktree 归属现场发现、不依赖扫描数据
	DefaultBranch string         `json:"defaultBranch"` // 远端默认分支（无 remote 时为空，可接受空值）
}

func (s *Service) Info(path string) (*Info, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	worktrees, err := git.WorktreeList(root)
	if err != nil {
		return nil, err
	}
	defaultBranch, _ := git.DefaultBranch(root) // 无 remote 返回空，可接受
	return &Info{
		Root:          root,
		Worktrees:     worktrees,
		DefaultBranch: defaultBranch,
	}, nil
}

// Refs 分支与 tag 列表，作为 git 树面板 / 双选交互的候选目标。
type Refs struct {
	Locals  []string           `json:"locals"`  // 本地分支名
	Current string             `json:"current"` // 当前检出分支（无检出为空）
	Remotes []git.RemoteBranch `json:"remotes"` // 远程分支（所有 remote）
	Tags    []string           `json:"tags"`    // 全部 tag 名
}

func (s *Service) Refs(path string) (*Refs, error) {
	root, ok := git.FindGitRoot(path)
	if !ok {
		return nil, fmt.Errorf("path 不是 git 仓库: path=%s", path)
	}
	locals, current, err := git.Branches(root)
	if err != nil {
		return nil, err
	}
	remotes, _ := git.RemoteBranches(root) // 无远程分支返回空，可接受
	tags, _ := git.Tags(root)              // 无 tag 返回空，可接受
	return &Refs{
		Locals:  locals,
		Current: current,
		Remotes: remotes,
		Tags:    tags,
	}, nil
}
