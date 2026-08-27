// Package testfixture 提供 cube 测试用的通用辅助：临时工作区、git 仓库 fixture、工程目录 fixture。
//
// 临时目录策略：写到系统临时目录（os.MkdirTemp），测试成功后自动删除；
// 测试失败时保留现场并在日志中打印路径，便于人工排查。
// （旧方案写项目内 runtime/test/ 且永不清理，会持续膨胀并被 goimports/IDE 扫描拖慢。）
//
// 用法：
//
//	ws := testfixture.NewWorkspace(t)
//	dir := ws.Dir  // 该测试专属目录（系统临时目录下 <testname>-<随机后缀>/）
//
// 该包是普通包（非 _test.go），可被任何测试包 import。
package testfixture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Workspace 表示一个测试的专属工作目录。
// 每个测试获得独立子目录（带时间戳 + 测试名），互不污染，失败后可翻看现场。
type Workspace struct {
	testing.TB
	Dir string // 该测试的工作目录（绝对路径）
}

// NewWorkspace 在系统临时目录下为当前测试创建一个独立工作目录。
// 目录名格式：cube-test-<sanitized testname>-<随机后缀>。
// 测试成功结束后自动删除；失败时保留并打印路径供排查。
func NewWorkspace(t testing.TB) *Workspace {
	t.Helper()
	name := sanitizeName(t.Name())
	dir, err := os.MkdirTemp("", "cube-test-"+name+"-")
	if err != nil {
		t.Fatalf("testfixture: 创建工作目录失败: %v", err)
	}
	ws := &Workspace{TB: t, Dir: dir}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("测试失败，现场保留于: %s", dir)
			return
		}
		_ = os.RemoveAll(dir)
	})
	return ws
}

// sanitizeName 把测试名（含 "/" 等）压成目录安全字符串。
func sanitizeName(name string) string {
	r := strings.NewReplacer(
		"/", "_",
		" ", "_",
		":", "",
		"\\", "_",
	)
	s := r.Replace(name)
	// 按 rune 截断：中文测试名按字节截会切坏 UTF-8，导致 mkdir 报 illegal byte sequence
	if runes := []rune(s); len(runes) > 60 {
		return string(runes[:60])
	}
	return s
}

// Join 拼接工作目录下的子路径。
func (w *Workspace) Join(elem ...string) string {
	return filepath.Join(append([]string{w.Dir}, elem...)...)
}

// Mkdir 在工作目录下建子目录。
func (w *Workspace) Mkdir(elem ...string) string {
	w.Helper()
	p := w.Join(elem...)
	if err := os.MkdirAll(p, 0755); err != nil {
		w.Fatalf("testfixture: 创建目录失败: %v", err)
	}
	return p
}

// WriteFile 在工作目录下写文件。
func (w *Workspace) WriteFile(path string, content []byte) {
	w.Helper()
	full := w.Join(path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		w.Fatalf("testfixture: 创建父目录失败: %v", err)
	}
	if err := os.WriteFile(full, content, 0644); err != nil {
		w.Fatalf("testfixture: 写文件失败: %v", err)
	}
}

// Cleanup 删除整个工作目录。通常不需要调（保留现场排查）。
func (w *Workspace) Cleanup() {
	_ = os.RemoveAll(w.Dir)
}

// MakeProjectDir 建一个「会被 cube 扫描识别为 project」的目录（默认含真实 git 仓库）。
// 通过 opts 调整：WithGodot / WithWorktree / WithDirty / WithoutGit。
// 返回 project 目录绝对路径。
func (w *Workspace) MakeProjectDir(relPath string, opts ...ProjectOption) string {
	w.Helper()
	spec := &projectDirSpec{}
	for _, o := range opts {
		o(spec)
	}

	dir := w.Mkdir(relPath)
	if spec.noGit {
		return dir
	}

	if spec.worktree {
		// 用 .git 文件模拟 worktree（让 checkProjectPath 识别为 worktree project）。
		// 注意：这只是让扫描识别，不能真跑 git 命令（真 worktree 需 git worktree add）。
		w.WriteFile(filepath.Join(relPath, ".git"), []byte("gitdir: /tmp/nonexistent/.git/worktrees/fake"))
	} else {
		BuildGitRepo(w.TB, dir, GitRepoSpec{MakeDirty: spec.dirty})
	}

	if spec.godot {
		w.WriteFile(filepath.Join(relPath, "game.godot"), []byte("godot project"))
	}
	return dir
}

// AssertFileExists 断言 ws 下 relPath 文件存在。
func (w *Workspace) AssertFileExists(relPath string) {
	w.Helper()
	full := w.Join(relPath)
	if _, err := os.Stat(full); err != nil {
		w.Fatalf("期望文件存在但缺失: %s (%v)", full, err)
	}
}

// AssertFileNotExists 断言 ws 下 relPath 不存在。
func (w *Workspace) AssertFileNotExists(relPath string) {
	w.Helper()
	full := w.Join(relPath)
	if _, err := os.Stat(full); !os.IsNotExist(err) {
		w.Fatalf("期望文件不存在但存在: %s", full)
	}
}

// MakeGitRepo 极简便捷方法：在 ws 下建一个规范名子目录，构造一个带 1 个空 commit 的干净 git 仓库。
// 返回仓库绝对路径。复杂场景用 BuildGitRepo + ws.Mkdir。
func (w *Workspace) MakeGitRepo(name string) string {
	w.Helper()
	dir := w.Mkdir(name)
	BuildGitRepo(w.TB, dir, GitRepoSpec{})
	return dir
}

// MakeGitRepoWith 在 ws 下建一个 git 仓库并按 spec 构造状态。
func (w *Workspace) MakeGitRepoWith(name string, spec GitRepoSpec) string {
	w.Helper()
	dir := w.Mkdir(name)
	BuildGitRepo(w.TB, dir, spec)
	return dir
}

// MakeWorktree 在 ws 下为 repoDir 建一个真实的 linked worktree（git worktree add），
// 检出新分支 branch。返回 worktree 目录绝对路径。
// 区别于 WithWorktree()：后者只写 .git 文件骗过扫描，不能真跑 git；本方法可被
// git.WorktreeList / gitcache 采集等真实读路径使用。
func (w *Workspace) MakeWorktree(repoDir, relPath, branch string) string {
	w.Helper()
	wtDir := w.Join(relPath)
	runGit(w.TB, repoDir, "worktree", "add", "-b", branch, wtDir)
	return wtDir
}
