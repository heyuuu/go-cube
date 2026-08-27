package workbench

// worktree / 分支写侧的 Service 层测试：预填路径、预检拒绝、主目录防护、
// 定向刷新回调触发。git 层行为已由 util/git 的测试覆盖，这里只测编排。

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cube/internal/testfixture"
	"cube/util/git"
)

// newWriteService 返回带记录回调的 Service：refreshed 收到刷新请求的主项目路径。
func newWriteService() (*Service, *[]string) {
	var refreshed []string
	s := NewService(func(path string) error {
		refreshed = append(refreshed, path)
		return nil
	})
	return s, &refreshed
}

func TestWorktreeAdd_PrefillAndRefresh(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	s, refreshed := newWriteService()

	created, err := s.WorktreeAdd(repo, "feat/x", "", "")
	if err != nil {
		t.Fatalf("WorktreeAdd 报错: %v", err)
	}
	want := filepath.Join(ws.Dir, "repo.worktrees", "feat-x")
	if real, err := filepath.EvalSymlinks(want); err == nil {
		want = real // git 输出经符号链接规范化（/var → /private/var），比较前同样求值
	}
	if created.Path != want {
		t.Errorf("预填路径不符: want %s, got %s", want, created.Path)
	}
	if created.Branch != "feat/x" {
		t.Errorf("分支不符: %+v", created)
	}
	if len(*refreshed) != 1 || (*refreshed)[0] != repo {
		t.Errorf("应定向刷新主项目一次: %v", *refreshed)
	}

	// 规范全名入参剥前缀
	if _, err := s.WorktreeAdd(repo, "refs/heads/full", "", ""); err != nil {
		t.Fatalf("全名分支报错: %v", err)
	}

	// 目标目录已存在且非空 → 中文错误
	os.WriteFile(filepath.Join(ws.Dir, "repo.worktrees", "feat-x", "keep.txt"), []byte("x"), 0o644)
	if _, err := s.WorktreeAdd(repo, "feat/x2", "", filepath.Join(ws.Dir, "repo.worktrees", "feat-x")); err == nil ||
		!strings.Contains(err.Error(), "已存在且非空") {
		t.Errorf("非空目标应报中文错误, got: %v", err)
	}
}

// 在 worktree 里发起写操作：刷新目标应归并到主仓库路径。
func TestWorktreeAdd_FromWorktreeRefreshesMain(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	wtDir := ws.MakeWorktree(repo, "wt-old", "old")
	s, refreshed := newWriteService()

	if _, err := s.WorktreeAdd(wtDir, "feat", "", ""); err != nil {
		t.Fatalf("从 worktree 发起 WorktreeAdd 报错: %v", err)
	}
	canonicalRepo, _ := filepath.EvalSymlinks(repo)
	if len(*refreshed) != 1 || (*refreshed)[0] != canonicalRepo {
		t.Errorf("刷新应归并到主仓库: %v", *refreshed)
	}
}

func TestWorktreeRemove(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	s, refreshed := newWriteService()

	// 主仓库目录无论 force 均拒绝
	if err := s.WorktreeRemove(repo, repo, true); err == nil ||
		!strings.Contains(err.Error(), "主仓库工作目录") {
		t.Errorf("删主目录应被拒绝, got: %v", err)
	}

	// 干净副本直接删
	clean := ws.MakeWorktree(repo, "wt-clean", "clean")
	if err := s.WorktreeRemove(repo, clean, false); err != nil {
		t.Fatalf("干净副本删除应成功: %v", err)
	}

	// 脏副本非 force 返回结构化拒绝，force 删净
	dirty := ws.MakeWorktree(repo, "wt-dirty", "dirty")
	os.WriteFile(filepath.Join(dirty, "new.txt"), []byte("uncommitted"), 0o644)
	err := s.WorktreeRemove(repo, dirty, false)
	var denied *WorktreeRemoveDenied
	if !errors.As(err, &denied) {
		t.Fatalf("脏副本非 force 应返回 *WorktreeRemoveDenied, got: %v", err)
	}
	if len(denied.Reasons) == 0 {
		t.Errorf("拒绝原因不应为空")
	}
	if err := s.WorktreeRemove(repo, dirty, true); err != nil {
		t.Fatalf("force 删除应成功: %v", err)
	}
	if _, err := os.Stat(dirty); !os.IsNotExist(err) {
		t.Errorf("目录应已删除")
	}

	if len(*refreshed) != 2 {
		t.Errorf("每次成功删除都应刷新: %v", len(*refreshed))
	}
}

func TestBranchDelete(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	wtDir := ws.MakeWorktree(repo, "wt-hot", "hotfix")
	s, refreshed := newWriteService()

	// 被检出的分支无论 force 均拒绝，且说明检出位置
	err := s.BranchDelete(repo, "hotfix", true)
	if err == nil || !strings.Contains(err.Error(), wtDir) {
		t.Errorf("被检出分支应拒绝并说明位置, got: %v", err)
	}
	err = s.BranchDelete(repo, currentBranchOf(t, repo), true)
	if err == nil || !strings.Contains(err.Error(), "主仓库") && !strings.Contains(err.Error(), repo) {
		t.Errorf("主目录当前分支应拒绝, got: %v", err)
	}

	// 建分支副本 → 删副本 → 未检出分支可正常删除并触发刷新
	wtTmp := ws.MakeWorktree(repo, "wt-tmp", "to-delete")
	if err := s.WorktreeRemove(repo, wtTmp, false); err != nil {
		t.Fatalf("临时副本删除失败: %v", err)
	}
	before := len(*refreshed)
	if err := s.BranchDelete(repo, "to-delete", false); err != nil {
		t.Fatalf("未检出分支删除应成功: %v", err)
	}
	if len(*refreshed) != before+1 {
		t.Errorf("删分支应触发一次刷新: before=%d after=%d", before, len(*refreshed))
	}
}

// currentBranchOf 取仓库当前分支名（主目录检出分支拒绝测试用）。
func currentBranchOf(t *testing.T, repo string) string {
	t.Helper()
	b := git.CurrentBranch(repo)
	if b == "" {
		t.Fatalf("当前分支为空")
	}
	return b
}

func TestBranchAdd(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	wtDir := ws.MakeWorktree(repo, "wt-src", "src")
	s, refreshed := newWriteService()

	if err := s.BranchAdd(repo, "feat", ""); err != nil {
		t.Fatalf("BranchAdd 报错: %v", err)
	}
	// 全名入参剥前缀；从 worktree 内发起刷新归并主仓库
	if err := s.BranchAdd(wtDir, "refs/heads/full", "src"); err != nil {
		t.Fatalf("全名 + 指定基点报错: %v", err)
	}
	// 主目录路径与 worktree 归并路径的规范化形态不同（/var vs /private/var），比较前求值
	if len(*refreshed) != 2 {
		t.Fatalf("两次都应触发刷新: %v", *refreshed)
	}
	for _, got := range *refreshed {
		canonical, _ := filepath.EvalSymlinks(got)
		want, _ := filepath.EvalSymlinks(repo)
		if canonical != want {
			t.Errorf("刷新目标应归并主仓库: got %s want %s", got, repo)
		}
	}
	if err := s.BranchAdd(repo, "feat", ""); err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Errorf("重名应中文报错, got: %v", err)
	}
}
