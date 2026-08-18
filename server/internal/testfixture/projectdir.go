package testfixture

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
