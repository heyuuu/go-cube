// ref 查询与解析的测试（被测实现在 refs.go；directGit/directAddRemote 供同包测试共用）。
package git

import (
	"os/exec"
	"reflect"
	"testing"

	"cube/internal/testfixture"
)

// TestCurrentBranch 当前检出分支短名：attached 剥前缀返回；detached、非仓库为空。
func TestCurrentBranch(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "develop"})
	if got := CurrentBranch(dir); got != "develop" {
		t.Fatalf("CurrentBranch = %q，期望 develop", got)
	}
	directGit(t, dir, "checkout", "--detach")
	if got := CurrentBranch(dir); got != "" {
		t.Fatalf("detached 时 CurrentBranch 应为空，实际 %q", got)
	}
	if got := CurrentBranch(ws.Mkdir("empty")); got != "" {
		t.Fatalf("非仓库 CurrentBranch 应为空，实际 %q", got)
	}
}

// TestCurrentBranch_HeadOnTag HEAD 被 symbolic-ref 挂到 heads 外（tag）时为空，
// 不得把 refs/tags/* 整串误当分支名（git branch --show-current 同口径输出空）。
func TestCurrentBranch_HeadOnTag(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Tags: []string{"v1.0"}})
	directGit(t, dir, "symbolic-ref", "HEAD", "refs/tags/v1.0")
	if got := CurrentBranch(dir); got != "" {
		t.Fatalf("HEAD 挂在 refs/tags/* 时应为空，实际 %q", got)
	}
}

// TestBuildRef 规范全名 → Ref 值对象：三棵子树解析、谓词判定、
// 符号指针与未知形态报错（BuildRefs 依赖该错误口径做跳过）。
func TestBuildRef(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Ref
		kind    string // local / remote / tag，断言谓词与字段一致
		wantErr bool
	}{
		{"本地分支", "refs/heads/develop",
			Ref{Name: "refs/heads/develop", ShortName: "develop", Branch: "develop"}, "local", false},
		{"带斜杠的本地分支", "refs/heads/feature/x",
			Ref{Name: "refs/heads/feature/x", ShortName: "feature/x", Branch: "feature/x"}, "local", false},
		{"tag", "refs/tags/v1.0",
			Ref{Name: "refs/tags/v1.0", ShortName: "v1.0"}, "tag", false},
		{"远程跟踪分支", "refs/remotes/origin/master",
			Ref{Name: "refs/remotes/origin/master", ShortName: "origin/master", Remote: "origin", Branch: "master"}, "remote", false},
		{"带斜杠的远程分支", "refs/remotes/origin/feature/x",
			Ref{Name: "refs/remotes/origin/feature/x", ShortName: "origin/feature/x", Remote: "origin", Branch: "feature/x"}, "remote", false},
		{"远端 HEAD 符号指针", "refs/remotes/origin/HEAD", Ref{}, "", true},
		{"只有 remote 无分支", "refs/remotes/origin", Ref{}, "", true},
		{"未知子树", "refs/stash", Ref{}, "", true},
		{"短名不被接受", "origin/master", Ref{}, "", true},
		{"空串", "", Ref{}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildRef(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("应报错: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			if got != tc.want {
				t.Errorf("BuildRef = %+v，期望 %+v", got, tc.want)
			}
			switch {
			case tc.kind == "local" && !got.IsLocal():
				t.Errorf("%+v 应判定为 local", got)
			case tc.kind == "remote" && !got.IsRemote():
				t.Errorf("%+v 应判定为 remote", got)
			case tc.kind == "tag" && !got.IsTag():
				t.Errorf("%+v 应判定为 tag", got)
			}
		})
	}
}

// TestRefs 全量 ref 清单：三类 namespace 的 Ref 值对象 + 远端 HEAD 符号指针剔除。
// current 属 HEAD 状态，由 TestHeadRef 单独覆盖。
func TestRefs(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{
		Branch: "develop",
		Tags:   []string{"v1.0"},
	})
	directGit(t, dir, "update-ref", "refs/remotes/origin/master", "refs/heads/develop")
	directGit(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/master")

	refs, err := Refs(dir)
	if err != nil {
		t.Fatalf("Refs 出错: %v", err)
	}
	if !reflect.DeepEqual(refs.Locals, []Ref{
		{Name: "refs/heads/develop", ShortName: "develop", Branch: "develop"},
	}) {
		t.Fatalf("locals = %+v", refs.Locals)
	}
	if !reflect.DeepEqual(refs.Remotes, []Ref{
		{Name: "refs/remotes/origin/master", ShortName: "origin/master", Remote: "origin", Branch: "master"},
	}) {
		t.Fatalf("remotes = %+v（origin/HEAD 符号指针应被剔除）", refs.Remotes)
	}
	if !reflect.DeepEqual(refs.Tags, []Ref{
		{Name: "refs/tags/v1.0", ShortName: "v1.0"},
	}) {
		t.Fatalf("tags = %+v", refs.Tags)
	}
}

// TestRefs_NonRepo 非仓库目录返回零值不报错（降级约定）。
func TestRefs_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	refs, err := Refs(ws.Mkdir("empty"))
	if err != nil {
		t.Fatalf("非仓库 Refs 不应报错: %v", err)
	}
	if refs.Locals != nil || refs.Remotes != nil || refs.Tags != nil {
		t.Fatalf("非仓库应返回零值: %+v", refs)
	}
}

// TestHeadRef attached → refs/heads 全名；detached / 非仓库 → 空串。
func TestHeadRef(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepoWith("repo", testfixture.GitRepoSpec{Branch: "develop"})
	if got := HeadRef(dir); got != "refs/heads/develop" {
		t.Fatalf("HeadRef = %q，期望 refs/heads/develop", got)
	}
	directGit(t, dir, "checkout", "--detach")
	if got := HeadRef(dir); got != "" {
		t.Fatalf("detached 时 HeadRef 应为空，实际 %q", got)
	}
	if got := HeadRef(ws.Mkdir("empty")); got != "" {
		t.Fatalf("非仓库 HeadRef 应为空，实际 %q", got)
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

	// Refs 解析：refs/remotes/origin/feature/fix-bug → Remote/Branch 拆分
	repoRefs, err := Refs(dir)
	if err != nil {
		t.Fatalf("Refs 出错: %v", err)
	}
	found := false
	for _, ref := range repoRefs.Remotes {
		if ref.Remote == "origin" && ref.Branch == "feature/fix-bug" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Refs.Remotes %v 不含 {origin, feature/fix-bug}", repoRefs.Remotes)
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
