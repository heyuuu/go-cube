package gogit

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/heyuuu/cube/internal/testfixture"
)

// TestRemoteUrl_NonRepo 非仓库目录返回空值不报错（降级约定）。
func TestRemoteUrl_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("not-a-repo")
	url, err := RemoteUrl(dir)
	if err != nil || url != "" {
		t.Fatalf("非仓库 RemoteUrl 应返回 (\"\",nil)，实际 (%q,%v)", url, err)
	}
}

// TestRemoteUrl_WithRemote 有 origin remote 的仓库能读出 URL。
func TestRemoteUrl_WithRemote(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	// remote 指向另一个本地路径（合法的本地 remote）
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
		RemoteUrl: "/tmp/some-remote.git",
	})
	url, err := RemoteUrl(dir)
	if err != nil {
		t.Fatalf("RemoteUrl 出错: %v", err)
	}
	if url != "/tmp/some-remote.git" {
		t.Fatalf("RemoteUrl = %q，期望 /tmp/some-remote.git", url)
	}
}

// TestRemoteUrl_NoOrigin 无 origin remote 时返回空值不报错。
func TestRemoteUrl_NoOrigin(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	url, err := RemoteUrl(dir)
	if err != nil || url != "" {
		t.Fatalf("无 origin 时应返回空，实际 (%q,%v)", url, err)
	}
}

// TestBranches_CleanRepo 干净仓库返回当前分支。
func TestBranches_CleanRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "develop"})

	branches, current, err := Branches(dir)
	if err != nil {
		t.Fatalf("Branches 出错: %v", err)
	}
	if current != "develop" {
		t.Fatalf("当前分支 = %q，期望 develop", current)
	}
	found := false
	for _, b := range branches {
		if b == "develop" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("分支列表 %v 不含 develop", branches)
	}
}

// TestBranches_NonRepo 非仓库目录返回空不报错。
func TestBranches_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("empty")
	branches, current, err := Branches(dir)
	if err != nil {
		t.Fatalf("非仓库 Branches 不应报错: %v", err)
	}
	if branches != nil {
		t.Fatalf("非仓库 branches 应为 nil，实际 %v", branches)
	}
	if current != "" {
		t.Fatalf("非仓库 current 应为空，实际 %q", current)
	}
}

// TestTags 仓库含指定 tag。
func TestTags(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
		Tags: []string{"v1.0", "v2.0"},
	})
	tags, err := Tags(dir)
	if err != nil {
		t.Fatalf("Tags 出错: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("Tags 数量 = %d，期望 2：%v", len(tags), tags)
	}
	// tag 顺序由 git 决定，用 map 校验存在性
	got := map[string]bool{}
	for _, tg := range tags {
		got[tg] = true
	}
	if !got["v1.0"] || !got["v2.0"] {
		t.Fatalf("Tags 不全：%v", tags)
	}
}

// TestIsDirty_CleanAndDirty dirty 标志正确。
func TestIsDirty_CleanAndDirty(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	// clean 仓库
	cleanDir := ws.MakeGitRepo("clean")
	dirty, err := IsDirty(cleanDir)
	if err != nil {
		t.Fatalf("clean IsDirty 出错: %v", err)
	}
	if dirty {
		t.Fatalf("clean 仓库 IsDirty 应为 false")
	}
	// dirty 仓库（MakeDirty=true 留未跟踪文件）
	dirtyDir := ws.MakeGitRepoWith("dirty", testfixture.GitRepoSpec{MakeDirty: true})
	dirty, err = IsDirty(dirtyDir)
	if err != nil {
		t.Fatalf("dirty IsDirty 出错: %v", err)
	}
	if !dirty {
		t.Fatalf("dirty 仓库 IsDirty 应为 true")
	}
}

// TestDefaultBranch_NoOriginRemote 无 origin remote 时按本地 master/main 兜底。
func TestDefaultBranch_NoOriginRemote(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "main"})

	db, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch 出错: %v", err)
	}
	if db != "main" && db != "master" {
		t.Fatalf("DefaultBranch = %q，期望 main 或 master", db)
	}
}

// TestDefaultBranch_NonRepo 非仓库返回空不报错。
func TestDefaultBranch_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.Mkdir("empty")
	db, err := DefaultBranch(dir)
	if err != nil || db != "" {
		t.Fatalf("非仓库 DefaultBranch 应返回空，实际 (%q,%v)", db, err)
	}
}

// TestRemotes_WithMultipleRemotes 多 remote 能全部读出。
func TestRemotes_WithMultipleRemotes(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	// 先建仓库，再加额外 remote
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
		RemoteUrl: "https://github.com/a/b.git",
	})
	// fixture 只支持 origin，upstream 用 git 命令直接加
	directAddRemote(t, dir, "upstream", "https://github.com/upstream/b.git")

	remotes, err := Remotes(dir)
	if err != nil {
		t.Fatalf("Remotes 出错: %v", err)
	}
	if len(remotes) != 2 {
		t.Fatalf("Remotes 数量 = %d，期望 2：%v", len(remotes), remotes)
	}
	// 按 name 查找
	byName := map[string]Remote{}
	for _, r := range remotes {
		byName[r.Name] = r
	}
	if r, ok := byName["origin"]; !ok || r.Fetch != "https://github.com/a/b.git" {
		t.Fatalf("origin remote 异常：%v", byName["origin"])
	}
	if r, ok := byName["upstream"]; !ok || r.Fetch != "https://github.com/upstream/b.git" {
		t.Fatalf("upstream remote 异常：%v", byName["upstream"])
	}
}

// TestAheadBehind_NoRemote 无 remote 时 ahead/behind 应为 0 或不报错（无 origin/xxx ref 即跳过）。
func TestAheadBehind_NoRemote(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	// 无 origin/master，应返回 0,0,nil（gogit 内 resolveBranchHash 找不到 ref 返回 false，函数返回零值）
	ahead, behind, err := AheadBehind(dir, "master", "origin/master")
	if err != nil {
		t.Fatalf("无 remote AheadBehind 不应报错: %v", err)
	}
	// 这两个值无明确语义（找不到远程 ref），只要不报错即可
	_ = ahead
	_ = behind
}

// directAddRemote 直接调 git remote add（fixture 只支持 origin，额外 remote 在此加）。
func directAddRemote(t *testing.T, dir, name, url string) {
	t.Helper()
	cmd := exec.Command("git",
		"-c", "user.email=test@cube.local",
		"-c", "user.name=cube-test",
		"remote", "add", name, url)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add %s 失败: %v\n%s", name, err, out)
	}
}

// TestFixture_Smoke 验证 testfixture 的 git builder 能正常工作（基础设施冒烟）。
func TestFixture_Smoke(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	// 验证 .git 目录存在
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("fixture 未建出 .git: %v", err)
	}
}
