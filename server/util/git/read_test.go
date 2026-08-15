package git

import (
	"os"
	"os/exec"
	"path/filepath"
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

// TestAheadBehind_NoRemote 无 remote 时（origin/xxx ref 不存在）返回 (0,0,nil)。
func TestAheadBehind_NoRemote(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	ahead, behind, err := AheadBehind(dir, "master", "origin/master")
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

// TestStatusFiles_States 验证各文件状态映射为 git status --short 的 XY 码：
// 未跟踪 / 暂存新增 / 暂存删除，以及按路径排序。
// （工作区修改 " M" 场景由 TestStatusFiles_WorktreeModified 覆盖。）
func TestStatusFiles_States(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	// 先提交一个已跟踪文件，再制造各类工作区状态
	writeAbsFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	writeAbsFile("tracked.txt", "v1")
	directGit(t, dir, "add", "tracked.txt")
	directGit(t, dir, "commit", "-m", "init tracked")

	writeAbsFile("tracked.txt", "v2")    // 工作区修改 → " M"
	writeAbsFile("untracked.txt", "new") // 未跟踪 → "??"
	writeAbsFile("added.txt", "added")   // 暂存新增 → "A "
	directGit(t, dir, "add", "added.txt")
	directGit(t, dir, "rm", "-f", "tracked.txt") // 暂存删除（含磁盘；文件有改动须 -f）→ "D "

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatalf("StatusFiles 出错: %v", err)
	}

	byPath := make(map[string]string, len(files))
	for _, f := range files {
		byPath[f.Path] = f.Code
	}
	for path, wantCode := range map[string]string{
		"untracked.txt": "??",
		"added.txt":     "A ",
		"tracked.txt":   "D ",
	} {
		if code, ok := byPath[path]; !ok || code != wantCode {
			t.Errorf("文件 %s 状态 = (%q, 存在=%v)，期望 %q（全部: %v）", path, code, ok, wantCode, byPath)
		}
	}

	// 排序断言：按展示路径升序
	for i := 1; i < len(files); i++ {
		if files[i-1].Path > files[i].Path {
			t.Errorf("StatusFiles 未按路径排序: %v", files)
			break
		}
	}
}

// TestStatusFiles_Rename 已暂存改名（git mv）合并为单条 R 行，展示 "旧 -> 新"。
func TestStatusFiles_Rename(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	if err := os.WriteFile(filepath.Join(dir, "old.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	directGit(t, dir, "add", "old.txt")
	directGit(t, dir, "commit", "-m", "init")
	directGit(t, dir, "mv", "old.txt", "new.txt")

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatalf("StatusFiles 出错: %v", err)
	}
	if len(files) != 1 || files[0].Code != "R " || files[0].Path != "old.txt -> new.txt" {
		t.Fatalf("期望单条 [R  old.txt -> new.txt]，实际 %v", files)
	}
}

// TestStatusFiles_WorktreeModified 已跟踪文件被修改但未暂存时，工作区列为 M（" M"）。
func TestStatusFiles_WorktreeModified(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}
	directGit(t, dir, "add", "tracked.txt")
	directGit(t, dir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v2"), 0644); err != nil {
		t.Fatalf("写文件失败: %v", err)
	}

	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatalf("StatusFiles 出错: %v", err)
	}
	if len(files) != 1 || files[0].Path != "tracked.txt" || files[0].Code != " M" {
		t.Fatalf("期望单条 [tracked.txt \" M\"]，实际 %v", files)
	}
}

// TestStatusFiles_CleanAndNonRepo 干净仓库与非仓库目录都返回空不报错（降级约定）。
func TestStatusFiles_CleanAndNonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)

	cleanDir := ws.MakeGitRepo("clean")
	files, err := StatusFiles(cleanDir)
	if err != nil || len(files) != 0 {
		t.Fatalf("干净仓库应返回空，实际 (%v, %v)", files, err)
	}

	nonRepo := ws.Mkdir("empty")
	files, err = StatusFiles(nonRepo)
	if err != nil || len(files) != 0 {
		t.Fatalf("非仓库目录应返回空不报错，实际 (%v, %v)", files, err)
	}
}

// TestStatusFiles_GlobalIgnore 全局忽略规则生效：被 ~/.gitconfig 的
// core.excludesFile 或 XDG 默认 ignore 匹配的文件不算 untracked、不算 dirty。
// 原生 git 子进程继承测试进程环境，通过 HOME / XDG_CONFIG_HOME 指向测试目录
// 隔离真实用户配置。
func TestStatusFiles_GlobalIgnore(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")

	// --- 场景一：~/.gitconfig 显式配置 core.excludesFile ---
	home := ws.Mkdir("home")
	ws.WriteFile("home/.gitignore_global", []byte("*.log\n.DS_Store\n"))
	ws.WriteFile("home/.gitconfig", []byte(
		"[core]\n\texcludesFile = "+ws.Join("home", ".gitignore_global")+"\n"))

	ws.WriteFile("repo/keep.txt", []byte("x"))  // 普通未跟踪 → 应出现
	ws.WriteFile("repo/debug.log", []byte("x")) // 全局忽略 → 不应出现
	ws.WriteFile("repo/.DS_Store", []byte("x")) // 全局忽略 → 不应出现

	t.Setenv("HOME", home)
	files, err := StatusFiles(dir)
	if err != nil {
		t.Fatalf("StatusFiles 出错: %v", err)
	}
	if len(files) != 1 || files[0].Path != "keep.txt" {
		t.Fatalf("全局忽略未生效，期望仅 [keep.txt]，实际 %v", files)
	}

	// 只有全局忽略文件的仓库不应误报 dirty
	cleanDir := ws.MakeGitRepo("only-ignored")
	ws.WriteFile("only-ignored/.DS_Store", []byte("x"))
	ws.WriteFile("only-ignored/a.log", []byte("x"))
	dirty, err := IsDirty(cleanDir)
	if err != nil {
		t.Fatalf("IsDirty 出错: %v", err)
	}
	if dirty {
		t.Fatalf("只含全局忽略文件的仓库 IsDirty 应为 false")
	}

	// --- 场景二：无 .gitconfig 时走 XDG 默认（$XDG_CONFIG_HOME/git/ignore）---
	// 空 HOME 下没有 excludesFile，*.log/.DS_Store 不再被忽略（正确的 git 语义），
	// 本场景只验证 XDG 的 skip.txt 被过滤。
	emptyHome := ws.Mkdir("empty-home")
	xdg := ws.Mkdir("xdg")
	ws.WriteFile("xdg/git/ignore", []byte("skip.txt\n"))

	ws.WriteFile("repo/skip.txt", []byte("x")) // XDG 忽略 → 不应出现

	t.Setenv("HOME", emptyHome)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	files, err = StatusFiles(dir)
	if err != nil {
		t.Fatalf("StatusFiles 出错: %v", err)
	}
	byPath := make(map[string]string, len(files))
	for _, f := range files {
		byPath[f.Path] = f.Code
	}
	if _, ok := byPath["skip.txt"]; ok {
		t.Fatalf("XDG 默认 ignore 未生效，skip.txt 不应出现: %v", files)
	}
	for _, want := range []string{"keep.txt", "debug.log", ".DS_Store"} {
		if _, ok := byPath[want]; !ok {
			t.Fatalf("文件 %s 应出现（空 HOME 下无全局规则）: %v", want, files)
		}
	}
}

// --- 纯函数表驱动测试 ---

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

// TestParseStatusPorcelain 解析 status --porcelain -z 输出：
// 普通行、rename/copy 双路径行、尾部 NUL、短记录跳过。
func TestParseStatusPorcelain(t *testing.T) {
	out := "?? a.txt\x00" + " M b.txt\x00" + "R  new.txt\x00old.txt\x00" + "C  c2.txt\x00c1.txt\x00" + "A  d.txt\x00"

	got := parseStatusPorcelain(out)
	want := []FileStatus{
		{Code: "??", Path: "a.txt"},
		{Code: " M", Path: "b.txt"},
		{Code: "R ", Path: "old.txt -> new.txt"},
		{Code: "C ", Path: "c1.txt -> c2.txt"},
		{Code: "A ", Path: "d.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseStatusPorcelain = %v，期望 %v", got, want)
	}
	if parseStatusPorcelain("") != nil {
		t.Fatalf("空输入应返回 nil")
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
