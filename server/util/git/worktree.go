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
