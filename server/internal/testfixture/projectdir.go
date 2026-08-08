package testfixture

import (
	"os"
	"path/filepath"
)

// ProjectOption 配置 MakeProjectDir 的构造行为。
type ProjectOption func(*projectDirSpec)

type projectDirSpec struct {
	godot    bool
	worktree bool
	noGit    bool
	dirty    bool
}

// WithGodot 在 project 目录放一个 game.godot 文件。
func WithGodot() ProjectOption { return func(s *projectDirSpec) { s.godot = true } }

// WithWorktree 用 .git 文件（非目录）模拟 worktree。
func WithWorktree() ProjectOption { return func(s *projectDirSpec) { s.worktree = true } }

// WithDirty 让该 git 仓库处于 dirty 状态（工作区留未跟踪文件）。
func WithDirty() ProjectOption { return func(s *projectDirSpec) { s.dirty = true } }

// WithoutGit 不放 .git（用于测非仓库目录的降级）。
func WithoutGit() ProjectOption { return func(s *projectDirSpec) { s.noGit = true } }

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
