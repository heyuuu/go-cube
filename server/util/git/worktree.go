package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Worktree 描述仓库的一个工作副本（主目录或 linked worktree）。
// 主目录与 linked worktree 共享同一对象库，但各有独立 HEAD/暂存区。
type Worktree struct {
	Path     string `json:"path"`     // 工作副本绝对路径
	Head     string `json:"head"`     // 当前 HEAD commit sha（detached 时也有值）
	Branch   string `json:"branch"`   // 检出分支短名（refs/heads/ 前缀已剥）；detached/bare 为空
	Bare     bool   `json:"bare"`     // 裸仓库（无工作区文件）
	Detached bool   `json:"detached"` // HEAD 游离（未检出任何分支）
}

// WorktreeList 返回 dir 所在仓库的全部工作副本（主目录在前）。
// 非空输出依赖 runOut 的稳定环境注入；调用方需先确认 dir 是 git 目录。
func WorktreeList(dir string) ([]Worktree, error) {
	out, err := runOut(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list 执行失败: %w", err)
	}
	return parseWorktreePorcelain(out), nil
}

// WorktreeMain 探测 dir（或其祖先）是否为 linked worktree，是则返回主仓库目录，否则返回空串。
// worktree 的 .git 是文件，内容形如 "gitdir: /主仓库/.git/worktrees/<名>"，从中截出主仓库目录。
// 提案 1032：worktree 不再是独立项目，此函数是「命中 worktree 目录 → 归并主项目」的探测原语。
func WorktreeMain(dir string) string {
	root, ok := FindGitRoot(dir)
	if !ok {
		return ""
	}
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil || !info.Mode().IsRegular() {
		return "" // .git 不存在或是目录 → 非 worktree
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	// gitdir 形如 /主仓库/.git/worktrees/<名>；找到 /.git/worktrees/ 截断。
	// git 写入的是符号链接规范化后的路径（macOS 上 /var → /private/var），
	// 与调用方持有的扫描路径可能不一致，返回前同样求值保持口径统一
	const marker = "/.git/worktrees/"
	if idx := strings.Index(gitdir, marker); idx >= 0 {
		if real, err := filepath.EvalSymlinks(gitdir[:idx]); err == nil {
			return real
		}
		return gitdir[:idx]
	}
	return ""
}

// WorktreeAdd 在 dir 仓库的 targetPath 处创建新工作副本，基点 commitish 为
// commit / 分支 / tag（空串 = HEAD）。branch 决定形态：
//   - 空串：detached（--detach，不创建分支）；
//   - 分支不存在：以 commitish 为起点新建分支（-b）；
//   - 分支已存在：检出该分支。
//
// 分支已被任一工作副本（含主目录）检出时 git 会拒绝，这里基于 WorktreeList
// 预检并直接给出中文错误（指明检出位置），避免依赖 stderr 文案。
// targetPath 已存在且非空同样由 git 拒绝，调用方（workbench Service）负责预校验。
// 成功后返回新副本的信息（含 Head / Branch），供上层直接发起 open。
func WorktreeAdd(dir string, targetPath string, branch string, commitish string) (*Worktree, error) {
	if branch != "" {
		if at, err := branchCheckedOutAt(dir, branch); err != nil {
			return nil, err
		} else if at != "" {
			return nil, fmt.Errorf("分支 %s 已被工作副本检出，不能重复检出: %s", branch, at)
		}
	}

	args := []string{"worktree", "add"}
	switch {
	case branch == "":
		args = append(args, "--detach", targetPath)
		if commitish != "" {
			args = append(args, commitish)
		}
	case branchExists(dir, branch):
		// 检出已有分支：分支名即位置参数基点，忽略 commitish
		args = append(args, targetPath, branch)
	default:
		args = append(args, "-b", branch, targetPath)
		if commitish != "" {
			args = append(args, commitish)
		}
	}
	if _, err := runOut(dir, args...); err != nil {
		return nil, fmt.Errorf("git worktree add 执行失败: %w", err)
	}

	// 从列表回读新副本信息；git 输出的路径经符号链接规范化，比较前同样求值
	canonicalTarget, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		canonicalTarget = targetPath
	}
	list, err := WorktreeList(dir)
	if err != nil {
		return nil, err
	}
	for i, wt := range list {
		if wt.Path == canonicalTarget {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("worktree 已创建但未出现在副本列表中: %s", targetPath)
}

// WorktreeRemove 删除 dir 仓库中 targetPath 处的工作副本（git worktree remove）。
// 非 force 时 git 自身拒绝删除含未提交改动/未跟踪文件的副本，错误经本包包装上抛；
// 上层（workbench Service）另有基于快照的中文预检，这里的拒绝只作兜底。
// 删除后的元数据清理由调用方统一 WorktreePrune 收尾。
func WorktreeRemove(dir string, targetPath string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, targetPath)
	if _, err := runOut(dir, args...); err != nil {
		return fmt.Errorf("git worktree remove 执行失败: %w", err)
	}
	return nil
}

// branchExists 判断分支是否已存在于本地 refs（不区分是否检出）。
// 判定失败按不存在降级——真正的冲突交由 git worktree add 报错兜底。
func branchExists(dir string, branch string) bool {
	refs, err := Refs(dir)
	if err != nil {
		return false
	}
	for _, r := range refs.Locals {
		if r.Branch == branch {
			return true
		}
	}
	return false
}

// branchCheckedOutAt 返回检出 branch 的工作副本路径（含主目录），未检出返回空串。
func branchCheckedOutAt(dir string, branch string) (string, error) {
	list, err := WorktreeList(dir)
	if err != nil {
		return "", fmt.Errorf("枚举工作副本失败: %w", err)
	}
	for _, wt := range list {
		if wt.Branch == branch {
			return wt.Path, nil
		}
	}
	return "", nil
}

// WorktreePrune 清理主仓库 root 中目录已不存在的 worktree 元数据记录。
// 幂等且无损：只删失效记录，不碰任何现存 worktree。doctor 的发现/修复共用。
func WorktreePrune(root string) error {
	if err := Run(root, "worktree", "prune"); err != nil {
		return fmt.Errorf("git worktree prune 执行失败: %w", err)
	}
	return nil
}

// parseWorktreePorcelain 解析 `git worktree list --porcelain` 输出：
// 每个工作副本一个块，块以 "worktree <path>" 起、空行止，属性行可选。
func parseWorktreePorcelain(out string) []Worktree {
	var result []Worktree
	var cur *Worktree
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			if cur != nil {
				result = append(result, *cur)
			}
			cur = &Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case cur == nil:
			// 块外的杂行（前导空行等），跳过
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), RefHeadsPrefix)
		case line == "detached":
			cur.Detached = true
		case line == "bare":
			cur.Bare = true
		}
	}
	if cur != nil {
		result = append(result, *cur)
	}
	return result
}
