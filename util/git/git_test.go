package git

import (
	"testing"

	"github.com/heyuuu/cube/internal/testfixture"
)

func TestFindGitRoot_AtRoot(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")

	root, ok := FindGitRoot(repo)
	if !ok {
		t.Fatalf("仓库根目录 FindGitRoot 应返回 ok=true")
	}
	if root != repo {
		t.Fatalf("FindGitRoot = %q，期望 %q", root, repo)
	}
}

func TestFindGitRoot_InSubdir(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	repo := ws.MakeGitRepo("repo")
	sub := ws.Mkdir("repo/src/deep/nested")

	root, ok := FindGitRoot(sub)
	if !ok {
		t.Fatalf("子目录 FindGitRoot 应返回 ok=true")
	}
	if root != repo {
		t.Fatalf("FindGitRoot = %q，期望 %q", root, repo)
	}
}

func TestFindGitRoot_NotARepo(t *testing.T) {
	// 注意：此 case 必须用系统临时目录而非 testfixture 的 runtime/test/，
	// 因为 runtime/test/ 在仓库内，FindGitRoot 向上探测会命中仓库根的 .git。
	dir := t.TempDir()
	_, ok := FindGitRoot(dir)
	if ok {
		t.Fatalf("非仓库目录 FindGitRoot 应返回 ok=false")
	}
}

func TestFindGitRoot_WorktreeStyleDotGitFile(t *testing.T) {
	// .git 是文件（worktree），FindGitRoot 只看 .git 存在与否，应能识别
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("wt")
	ws.WriteFile("wt/.git", []byte("gitdir: /somewhere"))

	root, ok := FindGitRoot(dir)
	if !ok || root != dir {
		t.Fatalf("worktree 风格 .git 文件 FindGitRoot 异常：root=%q ok=%v", root, ok)
	}
}
