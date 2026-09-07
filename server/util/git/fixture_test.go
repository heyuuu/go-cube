package git

// git 包自持的测试基建：建真实 .git 目录 / 按声明构造仓库状态。
// 从 internal/testfixture 复制裁剪而来（去掉了 project 扫描类 fixture）——
// git 包只依赖 git 本身，不反向依赖任何 cube 包，便于整包独立出去。
// testfixture 原件继续服务领域层 / 出口层测试。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// testWorkspace 表示一个测试的专属工作目录（系统临时目录下，成功自动删、失败留现场）。
type testWorkspace struct {
	testing.TB
	Dir string
}

// newTestWorkspace 在系统临时目录下为当前测试创建独立工作目录。
func newTestWorkspace(t testing.TB) *testWorkspace {
	t.Helper()
	dir, err := os.MkdirTemp("", "git-test-"+sanitizeName(t.Name())+"-")
	if err != nil {
		t.Fatalf("创建测试工作目录失败: %v", err)
	}
	ws := &testWorkspace{TB: t, Dir: dir}
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

func (w *testWorkspace) Join(elem ...string) string {
	return filepath.Join(append([]string{w.Dir}, elem...)...)
}

func (w *testWorkspace) Mkdir(elem ...string) string {
	w.Helper()
	p := w.Join(elem...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		w.Fatalf("创建目录失败: %v", err)
	}
	return p
}

func (w *testWorkspace) WriteFile(path string, content []byte) {
	w.Helper()
	full := w.Join(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		w.Fatalf("创建父目录失败: %v", err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		w.Fatalf("写文件失败: %v", err)
	}
}

// MakeGitRepo 极简便捷方法：建一个带 1 个空 commit 的干净 git 仓库。
func (w *testWorkspace) MakeGitRepo(name string) string {
	w.Helper()
	dir := w.Mkdir(name)
	buildGitRepo(w.TB, dir, GitRepoSpec{})
	return dir
}

// MakeGitRepoWith 建一个 git 仓库并按 spec 构造状态。
func (w *testWorkspace) MakeGitRepoWith(name string, spec GitRepoSpec) string {
	w.Helper()
	dir := w.Mkdir(name)
	buildGitRepo(w.TB, dir, spec)
	return dir
}

// MakeWorktree 为 repoDir 建一个真实的 linked worktree（git worktree add），检出新分支 branch。
func (w *testWorkspace) MakeWorktree(repoDir, relPath, branch string) string {
	w.Helper()
	wtDir := w.Join(relPath)
	runGit(w.TB, repoDir, "worktree", "add", "-b", branch, wtDir)
	return wtDir
}

// gitConfigArgs 默认 git 用户配置（避免依赖系统全局配置）。
var gitConfigArgs = []string{
	"-c", "user.email=test@cube.local",
	"-c", "user.name=cube-test",
	"-c", "commit.gpgsign=false",
}

// runGit 在 dir 下执行 git 命令，失败时 t.Fatalf。
// 通过 -c 注入 user 配置，确保不依赖系统全局 git config。
func runGit(t testing.TB, dir string, args ...string) {
	t.Helper()
	full := append([]string{}, gitConfigArgs...)
	full = append(full, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s 失败: %v\n输出: %s",
			strings.Join(args, " "), dir, err, out)
	}
}

// GitRepoSpec 描述一个测试 git 仓库的预期状态。
// 仅声明「想要的样子」，builder 负责构造到该状态。
type GitRepoSpec struct {
	// Branch 初始分支名。空串用 git 默认（master/main）。
	Branch string

	// EmptyCommitCount 仓库初始空 commit 数量（用 --allow-empty）。默认 1。
	EmptyCommitCount int

	// Tags 要打的 tag 列表（在初始 commit 上打）。
	Tags []string

	// RemoteUrl 配置的 origin remote 地址。空串表示不加 remote。
	RemoteUrl string

	// AheadBy 在初始 commit 之后，本地再追加多少个 commit（造成 ahead）。
	// 配合 RemoteUrl 模拟 origin remote（指向一个本地旧仓库）才有意义。
	AheadBy int

	// MakeDirty 为 true 时，builder 结束后在工作区留一个未提交改动（dirty 状态）。
	MakeDirty bool
}

// buildGitRepo 在 dir 处按 spec 构造一个真实 git 仓库。dir 必须已存在。
func buildGitRepo(t testing.TB, dir string, spec GitRepoSpec) {
	t.Helper()

	count := spec.EmptyCommitCount
	if count < 1 {
		count = 1
	}

	runGit(t, dir, "init")
	if spec.Branch != "" {
		// init 后改默认分支名（尚未 commit，可直接改）
		runGit(t, dir, "checkout", "-b", spec.Branch)
	}
	for i := 0; i < count; i++ {
		msg := fmt.Sprintf("commit %d", i+1)
		runGit(t, dir, "commit", "--allow-empty", "-m", msg)
	}
	for _, tag := range spec.Tags {
		runGit(t, dir, "tag", tag)
	}
	if spec.RemoteUrl != "" {
		runGit(t, dir, "remote", "add", "origin", spec.RemoteUrl)
	}
	for i := 0; i < spec.AheadBy; i++ {
		runGit(t, dir, "commit", "--allow-empty", "-m", fmt.Sprintf("ahead %d", i+1))
	}
	if spec.MakeDirty {
		// 写一个未跟踪文件，制造 dirty（不改已跟踪文件，避免改变 ahead/behind 语义）
		writeFile(t, filepath.Join(dir, "dirty-flag.txt"), []byte("uncommitted"))
	}
}

func writeFile(t testing.TB, path string, content []byte) {
	t.Helper()
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %q", path))
	cmd.Stdin = strings.NewReader(string(content))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("写文件 %s 失败: %v\n%s", path, err, out)
	}
}
