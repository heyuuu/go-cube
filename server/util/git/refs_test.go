// ref 查询与解析的测试（被测实现在 refs.go；directGit/directAddRemote 供同包测试共用）。
package git

import (
	"os/exec"
	"reflect"
	"testing"

	"cube/internal/testfixture"
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

// TestBranches_DetachedHead detached HEAD 时当前分支为空串（symbolic-ref 失败降级）。
func TestBranches_DetachedHead(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	directGit(t, dir, "checkout", "--detach")

	_, current, err := Branches(dir)
	if err != nil {
		t.Fatalf("Branches 出错: %v", err)
	}
	if current != "" {
		t.Fatalf("detached HEAD 时 current 应为空，实际 %q", current)
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

// TestDefaultBranch_OriginHEAD origin/HEAD 已设置时直接取其指向的分支。
func TestDefaultBranch_OriginHEAD(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "develop"})
	directGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")

	db, err := DefaultBranch(dir)
	if err != nil {
		t.Fatalf("DefaultBranch 出错: %v", err)
	}
	if db != "develop" {
		t.Fatalf("DefaultBranch = %q，期望 develop（origin/HEAD 指向）", db)
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

// TestRemotes_SeparatePushUrl 配置独立 pushurl 时 Fetch 与 Push 不同。
func TestRemotes_SeparatePushUrl(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
		RemoteUrl: "https://github.com/a/b.git",
	})
	directGit(t, dir, "remote", "set-url", "--push", "origin", "git@github.com:a/b.git")

	remotes, err := Remotes(dir)
	if err != nil || len(remotes) != 1 {
		t.Fatalf("Remotes = (%v, %v)，期望单个 remote", remotes, err)
	}
	r := remotes[0]
	if r.Fetch != "https://github.com/a/b.git" || r.Push != "git@github.com:a/b.git" {
		t.Fatalf("pushurl remote 异常：%+v", r)
	}
}

// TestAheadBehindRemote_NoRemote 无 remote 时（origin/xxx ref 不存在）返回 (0,0,nil)。
func TestAheadBehindRemote_NoRemote(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	ahead, behind, err := AheadBehindRemote(dir, "master", "origin", "master")
	if err != nil {
		t.Fatalf("无 remote AheadBehind 不应报错: %v", err)
	}
	if ahead != 0 || behind != 0 {
		t.Fatalf("无 remote AheadBehind 应为 (0,0)，实际 (%d,%d)", ahead, behind)
	}
}

// TestAheadBehindRemote_Diverged 分叉场景：ahead/behind 方向正确（本地独有 / 远端独有）。
func TestAheadBehindRemote_Diverged(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "master"})

	// 同步点 → 本地 master 加 1 commit；base 分支上加另 1 commit 当作远端状态
	directGit(t, dir, "update-ref", "refs/remotes/origin/master", "refs/heads/master")
	directGit(t, dir, "branch", "base")
	directGit(t, dir, "commit", "--allow-empty", "-m", "local ahead")
	directGit(t, dir, "checkout", "base")
	directGit(t, dir, "commit", "--allow-empty", "-m", "remote ahead")
	directGit(t, dir, "update-ref", "refs/remotes/origin/master", "HEAD")
	directGit(t, dir, "checkout", "master")

	ahead, behind, err := AheadBehindRemote(dir, "master", "origin", "master")
	if err != nil {
		t.Fatalf("AheadBehindRemote 出错: %v", err)
	}
	if ahead != 1 || behind != 1 {
		t.Fatalf("ahead/behind = %d/%d，期望 1/1（双方各独有 1 个 commit）", ahead, behind)
	}
}

// TestBranches_OnlyLocalRefs 带斜杠的本地分支（feature/fix-bug）必须返回，
// 远程跟踪引用（refs/remotes/origin/*）不得混入（push 依赖此约定选本地分支）。
func TestBranches_OnlyLocalRefs(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{RemoteUrl: "/tmp/some-remote.git"})

	_, current, err := Branches(dir)
	if err != nil {
		t.Fatalf("Branches 出错: %v", err)
	}
	// 建带斜杠的本地分支 + 造一个远程跟踪引用（比 fetch 轻，refs 层面等价）
	directGit(t, dir, "branch", "feature/fix-bug")
	directGit(t, dir, "update-ref", "refs/remotes/origin/"+current, "refs/heads/"+current)

	branches, _, err := Branches(dir)
	if err != nil {
		t.Fatalf("Branches 出错: %v", err)
	}
	hasSlashBranch := false
	for _, b := range branches {
		if b == "origin/"+current {
			t.Fatalf("分支列表 %v 混入了远程跟踪分支 origin/%s", branches, current)
		}
		if b == "feature/fix-bug" {
			hasSlashBranch = true
		}
	}
	if !hasSlashBranch {
		t.Fatalf("分支列表 %v 不含带斜杠的本地分支 feature/fix-bug", branches)
	}
}

// TestAheadBehindRemote_SlashBranch 斜杠分支（feature/fix-bug）端到端：
// 本地领先 remote 2 个 commit；同时验证 RemoteBranches 对斜杠远程分支的解析
// （info -v 分支同步宽表按短名交集 + 每格调 AheadBehindRemote，依赖这两个行为）。
func TestAheadBehindRemote_SlashBranch(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "master"})

	// remote 跟踪引用停在 master 当前位置，本地 feature/fix-bug 在其上追加 2 commit
	directGit(t, dir, "branch", "feature/fix-bug")
	directGit(t, dir, "update-ref", "refs/remotes/origin/feature/fix-bug", "refs/heads/master")
	directGit(t, dir, "checkout", "feature/fix-bug")
	directGit(t, dir, "commit", "--allow-empty", "-m", "ahead 1")
	directGit(t, dir, "commit", "--allow-empty", "-m", "ahead 2")

	ahead, behind, err := AheadBehindRemote(dir, "feature/fix-bug", "origin", "feature/fix-bug")
	if err != nil {
		t.Fatalf("AheadBehindRemote 出错: %v", err)
	}
	if ahead != 2 || behind != 0 {
		t.Fatalf("ahead/behind = %d/%d，期望 2/0", ahead, behind)
	}

	// RemoteBranches 解析：refs/remotes/origin/feature/fix-bug → {origin, feature/fix-bug}
	remoteBranches, err := RemoteBranches(dir)
	if err != nil {
		t.Fatalf("RemoteBranches 出错: %v", err)
	}
	found := false
	for _, rb := range remoteBranches {
		if rb.Remote == "origin" && rb.Branch == "feature/fix-bug" {
			found = true
		}
	}
	if !found {
		t.Fatalf("RemoteBranches %v 不含 {origin, feature/fix-bug}", remoteBranches)
	}
}

// directGit 在 dir 下直接调 git（带测试 user 配置），fixture 未覆盖的场景用。
func directGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git",
		append([]string{"-c", "user.email=test@cube.local", "-c", "user.name=cube-test"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s 失败: %v\n%s", args, dir, err, out)
	}
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

// TestParseRemotesVerbose 解析 git remote -v 输出：fetch/push 合并、
// 独立 pushurl、同名多行取第一条、无 TAB 的行跳过、按名排序。
func TestParseRemotesVerbose(t *testing.T) {
	out := "origin\tgit@github.com:a/b.git (fetch)\n" +
		"origin\tgit@github.com:a/b.git (push)\n" +
		"upstream\thttps://x/y.git (fetch)\n" +
		"upstream\tgit@x:y.git (push)\n" +
		"garbage-no-tab\n" +
		"\n"

	got := parseRemotesVerbose(out)
	want := []Remote{
		{Name: "origin", Fetch: "git@github.com:a/b.git", Push: "git@github.com:a/b.git"},
		{Name: "upstream", Fetch: "https://x/y.git", Push: "git@x:y.git"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseRemotesVerbose = %v，期望 %v", got, want)
	}
	if parseRemotesVerbose("") != nil {
		t.Fatalf("空输入应返回 nil")
	}
}

// TestParseCountPair 解析 rev-list --left-right --count 的 "ahead\tbehind" 输出。
func TestParseCountPair(t *testing.T) {
	cases := []struct {
		in     string
		ahead  int
		behind int
		ok     bool
	}{
		{"2\t0\n", 2, 0, true},
		{"0\t3\n", 0, 3, true},
		{"12\t34", 12, 34, true},
		{"1", 0, 0, false},      // 缺 tab
		{"a\tb\n", 0, 0, false}, // 非数字
		{"", 0, 0, false},
	}
	for _, c := range cases {
		ahead, behind, ok := parseCountPair(c.in)
		if ok != c.ok || ahead != c.ahead || behind != c.behind {
			t.Errorf("parseCountPair(%q) = (%d,%d,%v)，期望 (%d,%d,%v)",
				c.in, ahead, behind, ok, c.ahead, c.behind, c.ok)
		}
	}
}

// TestSplitRemoteBranchShortName 远程分支短名拆分。
func TestSplitRemoteBranchShortName(t *testing.T) {
	cases := []struct {
		in         string
		wantRemote string
		wantBranch string
		wantOK     bool
	}{
		{"origin/master", "origin", "master", true},
		{"origin/feature/x", "origin", "feature/x", true},
		{"upstream/main", "upstream", "main", true},
		{"master", "", "", false}, // 无 remote 前缀
		{"", "", "", false},       // 空
		{"/foo", "", "", false},   // remote 名为空（idx<=0）
	}
	for _, c := range cases {
		remote, branch, ok := splitRemoteBranchShortName(c.in)
		if ok != c.wantOK || remote != c.wantRemote || branch != c.wantBranch {
			t.Errorf("splitRemoteBranchShortName(%q) = (%q,%q,%v)，期望 (%q,%q,%v)",
				c.in, remote, branch, ok, c.wantRemote, c.wantBranch, c.wantOK)
		}
	}
}

// TestFirstLine 取输出首行。
func TestFirstLine(t *testing.T) {
	cases := map[string]string{
		"git@x:a/b\nsecond\n": "git@x:a/b",
		"only\n":              "only",
		"":                    "",
		"  \n":                "",
	}
	for in, want := range cases {
		if got := firstLine(in); got != want {
			t.Errorf("firstLine(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestHeadSha 返回 HEAD 完整 sha；非仓库降级为空值。
func TestHeadSha(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{EmptyCommitCount: 2})

	sha, err := HeadSha(dir)
	if err != nil {
		t.Fatalf("HeadSha 出错: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("HEAD sha 长度 = %d，期望 40（%q）", len(sha), sha)
	}

	if sha, err := HeadSha(ws.Mkdir("not-a-repo")); err != nil || sha != "" {
		t.Errorf("非仓库 HeadSha 应返回 (\"\",nil)，实际 (%q,%v)", sha, err)
	}
}

// TestParentSha 父提交解析正确；根提交无父时报错（调用方决定降级）。
func TestParentSha(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{EmptyCommitCount: 2})

	head, err := HeadSha(dir)
	if err != nil {
		t.Fatalf("HeadSha 出错: %v", err)
	}
	parent, err := ParentSha(dir, head)
	if err != nil {
		t.Fatalf("ParentSha 出错: %v", err)
	}
	if len(parent) != 40 || parent == head {
		t.Errorf("父提交 sha 异常: %q", parent)
	}
	// EmptyCommitCount=2 时 parent 已是根提交：再取父应报错
	if _, err := ParentSha(dir, parent); err == nil {
		t.Error("根提交再取父应报错")
	}
}
