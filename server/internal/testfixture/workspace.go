// Package testfixture 提供 cube 测试用的通用辅助：临时工作区、git 仓库 fixture、工程目录 fixture。
//
// 临时目录策略：默认写到项目根的 runtime/test/ 下（已 gitignore），而非系统 /tmp。
// 理由：测试失败时方便人工翻看现场排查；runtime/test/ 持久保留，不自动清理。
//
// 用法：
//
//	ws := testfixture.NewWorkspace(t)
//	dir := ws.Dir  // 该测试专属目录（runtime/test/<毫秒时间戳>-<testname>/）
//
// 该包是普通包（非 _test.go），可被任何测试包 import。
package testfixture

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// testRootOnce 保证 runtime/test 目录只创建一次。
var (
	testRootOnce sync.Once
	testRootErr  error
	testRootPath string
)

// ensureTestRoot 确保 runtime/test 存在，返回其绝对路径。
// 依赖项目根定位（testfixture 包位于 <root>/internal/testfixture，向上两级即项目根）。
func ensureTestRoot() (string, error) {
	testRootOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			testRootErr = fmt.Errorf("无法定位 testfixture 源文件")
			return
		}
		// file = <root>/internal/testfixture/workspace.go
		root := filepath.Join(filepath.Dir(file), "..", "..")
		testRootPath = filepath.Join(root, "runtime", "test")
		if err := os.MkdirAll(testRootPath, 0755); err != nil {
			testRootErr = fmt.Errorf("创建测试根目录失败: %w", err)
			return
		}
	})
	return testRootPath, testRootErr
}

// Workspace 表示一个测试的专属工作目录。
// 每个测试获得独立子目录（带时间戳 + 测试名），互不污染，失败后可翻看现场。
type Workspace struct {
	testing.TB
	Dir string // 该测试的工作目录（绝对路径）
}

// NewWorkspace 在 runtime/test/ 下为当前测试创建一个独立工作目录。
// 目录名格式：<毫秒时间戳>-<sanitized testname>，便于排序与定位。
// 目录不自动清理（排查用）；如需清理可在测试末尾调 ws.Cleanup()。
func NewWorkspace(t testing.TB) *Workspace {
	t.Helper()
	root, err := ensureTestRoot()
	if err != nil {
		t.Fatalf("testfixture: %v", err)
	}
	name := sanitizeName(t.Name())
	dirName := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405.000"), name)
	dir := filepath.Join(root, dirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("testfixture: 创建工作目录失败: %v", err)
	}
	return &Workspace{TB: t, Dir: dir}
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
	if len(s) > 60 {
		s = s[:60]
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
