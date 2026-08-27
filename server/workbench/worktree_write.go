package workbench

// worktree / 分支写侧（提案 1031）：新增 worktree、删除 worktree、删除分支。
// Service 入口方法在 service.go（薄编排），本文件是预检、预填路径推导等主题逻辑，
// 以包级函数暴露。git 子进程调用全部走 util/git 类型化函数（AGENTS.md 规则 14）。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cube/util/git"
)

// WorktreeCreated 新增 worktree 的结果：前端可直接对新路径发起 open / 切换 TreeSource。
type WorktreeCreated struct {
	Path     string `json:"path"`     // 新 worktree 绝对路径（git 输出经符号链接规范化）
	Branch   string `json:"branch"`   // 检出分支短名；detached 为空
	Detached bool   `json:"detached"` // HEAD 游离
}

// WorktreeRemoveDenied 非 force 删除被预检拒绝：携带结构化原因列表，UI 据此
// 二次确认后升级 force 重试（决策 2：force 一步可达，拒绝不是死路而是提示）。
type WorktreeRemoveDenied struct {
	Reasons []string
}

func (e *WorktreeRemoveDenied) Error() string {
	return "存在以下未保存内容，需 force 才能删除: " + strings.Join(e.Reasons, "；")
}

// mainRootOf 返回 root 所属仓库的主目录（root 是 linked worktree 时定位主仓库，
// 否则即自身）。projcache 快照按主项目路径 key，写后定向刷新必须用它。
func mainRootOf(root string) string {
	if m := git.WorktreeMain(root); m != "" {
		return m
	}
	return root
}

// prefillWorktreePath 推导新 worktree 的默认路径：repo 同级容器
// <repoName>.worktrees/<分支名>/（决策 1）。分支名含 / 时拍平成 -（目录层级
// 处理属实现细节）；detached（branch 为空）回退用 commitish 短名，再退 "detached"。
func prefillWorktreePath(mainRoot string, branch string, commitish string) string {
	segment := sanitizeDirSegment(branch)
	if segment == "" {
		segment = sanitizeDirSegment(commitish)
	}
	if segment == "" {
		segment = "detached"
	}
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+".worktrees", segment)
}

// sanitizeDirSegment 把 ref 名拍平成单层目录名：剥 refs/heads/ 等前缀、/ 换成 -。
func sanitizeDirSegment(ref string) string {
	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimSpace(ref)
	return strings.ReplaceAll(ref, "/", "-")
}

// checkTargetDir 校验目标目录满足 git worktree add 的要求（不存在或为空目录）。
// 存在且非空时给中文错误，避免 git 的 stderr 直出。
func checkTargetDir(targetPath string) error {
	info, err := os.Stat(targetPath)
	if err != nil || !info.IsDir() {
		return nil // 不存在（或不是目录，git 自己会报）均放行
	}
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return fmt.Errorf("读取目标目录失败: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("目标目录已存在且非空: %s", targetPath)
	}
	return nil
}

// worktreeRemoveBlockers 收集非 force 删除 worktree 前的预检拒绝原因
// （未提交改动 / 未跟踪文件 / 未推送提交）。返回空切片表示可安全删除。
func worktreeRemoveBlockers(targetPath string) ([]string, error) {
	st, err := git.LoadRepoStatus(targetPath)
	if err != nil {
		// 预检失败不静默放行——删除不可逆，宁可让 force 路径兜底
		return nil, fmt.Errorf("预检工作副本状态失败: %w", err)
	}
	var reasons []string
	if st.Staged > 0 || st.Unstaged > 0 {
		reasons = append(reasons, fmt.Sprintf("有未提交改动（暂存 %d / 未暂存 %d 个文件）", st.Staged, st.Unstaged))
	}
	if st.Untracked > 0 {
		reasons = append(reasons, fmt.Sprintf("有 %d 个未跟踪文件", st.Untracked))
	}
	if st.Ahead > 0 {
		reasons = append(reasons, fmt.Sprintf("领先上游 %d 个提交未推送", st.Ahead))
	}
	return reasons, nil
}
