// 工作区状态查询的测试（被测实现在 status.go；建仓用 directGit，见 refs_test.go）。
package git

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"cube/internal/testfixture"
)

// TestLoadRepoStatus 汇总状态：分支 / sha / dirty / 分类计数；非仓库降级为零值。
func TestLoadRepoStatus(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	// clean 仓库
	cleanDir := ws.MakeGitRepoWith("clean", testfixture.GitRepoSpec{Branch: "develop"})
	st, err := LoadRepoStatus(cleanDir)
	if err != nil {
		t.Fatalf("clean LoadRepoStatus 出错: %v", err)
	}
	if st.Dirty || st.Staged != 0 || st.Unstaged != 0 || st.Untracked != 0 {
		t.Fatalf("clean 仓库应为全零: %+v", st)
	}
	if st.Branch != "develop" || st.Detached {
		t.Fatalf("分支应为 develop: %+v", st)
	}
	if len(st.Sha) != 40 {
		t.Errorf("HEAD sha 长度 = %d，期望 40", len(st.Sha))
	}

	// dirty 仓库（MakeDirty=true 留未跟踪文件）
	dirtyDir := ws.MakeGitRepoWith("dirty", testfixture.GitRepoSpec{MakeDirty: true})
	st, err = LoadRepoStatus(dirtyDir)
	if err != nil {
		t.Fatalf("dirty LoadRepoStatus 出错: %v", err)
	}
	if !st.Dirty || st.Untracked != 1 {
		t.Fatalf("dirty 仓库应 Untracked=1: %+v", st)
	}

	// 暂存 + 工作区修改的分类计数
	mixedDir := ws.MakeGitRepo("mixed")
	if err := os.WriteFile(filepath.Join(mixedDir, "a.txt"), []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	directGit(t, mixedDir, "add", "a.txt")
	directGit(t, mixedDir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(mixedDir, "a.txt"), []byte("v2"), 0644); err != nil { // 工作区修改
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mixedDir, "b.txt"), []byte("new"), 0644); err != nil { // 暂存新增
		t.Fatal(err)
	}
	directGit(t, mixedDir, "add", "b.txt")
	st, err = LoadRepoStatus(mixedDir)
	if err != nil {
		t.Fatalf("mixed LoadRepoStatus 出错: %v", err)
	}
	if st.Staged != 1 || st.Unstaged != 1 || st.Untracked != 0 || !st.Dirty {
		t.Fatalf("mixed 仓库应 Staged=1 Unstaged=1: %+v", st)
	}

	// 非仓库降级为零值
	st, err = LoadRepoStatus(ws.Mkdir("not-a-repo"))
	if err != nil || st == nil || st.Dirty {
		t.Fatalf("非仓库应返回零值 + nil，实际 (%+v, %v)", st, err)
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
	st, err := LoadRepoStatus(cleanDir)
	if err != nil {
		t.Fatalf("LoadRepoStatus 出错: %v", err)
	}
	if st.Dirty {
		t.Fatalf("只含全局忽略文件的仓库应为 clean")
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

// TestLoadIgnored 忽略目录与文件分开收集，路径相对仓库根（subDir 收窄时仍带前缀）。
func TestLoadIgnored(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	dir := ws.MakeGitRepo("repo")
	writeAbsFile := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("建目录失败: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	writeAbsFile(".gitignore", "node_modules/\n*.log\n")
	writeAbsFile("keep.txt", "x")
	writeAbsFile("debug.log", "x")
	writeAbsFile("node_modules/pkg.js", "x")
	writeAbsFile("sub/inner.log", "x")
	writeAbsFile("sub/keep.txt", "x")

	ig, err := LoadIgnored(dir, "")
	if err != nil {
		t.Fatalf("LoadIgnored 出错: %v", err)
	}
	if !ig.Dirs["node_modules"] {
		t.Errorf("node_modules 应在忽略目录中（Dirs=%v）", ig.Dirs)
	}
	if !ig.Files["debug.log"] || !ig.Files["sub/inner.log"] {
		t.Errorf("忽略文件不全（Files=%v）", ig.Files)
	}
	if ig.Has("keep.txt") || ig.Has("sub/keep.txt") {
		t.Error("未被忽略的文件不应命中")
	}
	if !ig.Has("node_modules") {
		t.Error("node_modules 应命中（目录级）")
	}

	// subDir 收窄：返回路径仍相对仓库根（与 treeFs 的拼接语义一致）
	igSub, err := LoadIgnored(dir, "sub")
	if err != nil {
		t.Fatalf("LoadIgnored(sub) 出错: %v", err)
	}
	if !igSub.Files["sub/inner.log"] || igSub.Has("sub/keep.txt") {
		t.Errorf("subDir 收窄结果不符（Files=%v）", igSub.Files)
	}
}

// TestLoadIgnored_NonRepo 非仓库目录返回空集合不报错（降级约定）。
func TestLoadIgnored_NonRepo(t *testing.T) {
	ws := testfixture.NewWorkspace(t)
	ig, err := LoadIgnored(ws.Mkdir("not-a-repo"), "")
	if err != nil || len(ig.Dirs) != 0 || len(ig.Files) != 0 {
		t.Fatalf("非仓库应返回空集合，实际 (%v, %v)", ig, err)
	}
}
