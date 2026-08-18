package testfixture

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// BuildGitRepo 在 dir 处按 spec 构造一个真实 git 仓库。
// dir 必须已存在（用 ws.Mkdir 创建）。
func BuildGitRepo(t testing.TB, dir string, spec GitRepoSpec) {
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
