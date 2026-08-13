package git

// 本包封装「写操作」的系统 git 命令调用，以及与具体 git 库无关的 git 辅助能力，
// 与 gogit 包中的 go-git 纯 Go 读实现相对。
//
// 为什么写操作要用系统 git 子进程（而不是统一走 go-git）：
//   - 写操作（clone / init / add / commit 等）需要透传 stdout/stderr，复用用户本地的
//     git 配置（凭据、SSH agent、hooks、alias、protocol 等），并支持交互式进度输出。
//   - 典型场景如 clone：需要 SSH/HTTPS 凭据助手、git-credential-osxkeychain 等本地
//     git 生态，go-git 在这些场景下兼容性差、体验差，而系统 git 能原生复用。
//   - 写操作调用频率低（相对于读），子进程的 fork/exec 开销可接受，换来的是完整的
//     本地 git 生态兼容。
//
// 此外，与具体 git 库无关的 git 辅助能力也归在本包：
//   - FindGitRoot（按 .git 探测仓库根）、ParseRepoUrl（解析 SSH/HTTPS 仓库地址）等。
//     它们不依赖任何 git 实现，只与 git 的概念/约定相关。
//
// 读操作（branches / ahead-behind / status 等）见独立的 gogit 包（util/gogit）。
// 本包刻意「只写不读」：需要读 git 仓库信息时请用 gogit 包。

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// Run 在指定工作目录下执行系统 git 命令，stdout/stderr 透传给当前终端。
//
// dir 为空串时表示在当前进程工作目录执行。本包内所有「写操作 / 需要透传输出的操作」
// （Clone / Init / Add / Commit 等）都基于此函数，统一 stdio 接管与 slog 记录。
func Run(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	slog.Info("执行命令", "cmd", cmd.String())
	return cmd.Run()
}

// Clone 使用系统 git 克隆仓库到 localPath，stdout/stderr 透传给当前终端。
// 参数：
//   - localPath: 克隆目标目录（绝对路径）
//   - repoUrl:   仓库地址（SSH 或 HTTPS）
//   - depth:     克隆深度，<=0 表示不限制（完整克隆）
//   - branch:    指定分支名，空串表示克隆默认分支
func Clone(localPath string, repoUrl string, depth int, branch string) error {
	args := []string{"clone", repoUrl, localPath}
	if depth > 0 {
		args = append(args, "--depth="+strconv.Itoa(depth))
	}
	if branch != "" {
		args = append(args, "--branch="+branch)
	}
	return Run("", args...)
}

// Init 在 dir 目录初始化一个新的 git 仓库（git init）。
func Init(dir string) error {
	return Run(dir, "init")
}

// Add 在 dir 仓库中暂存指定路径（git add <paths...>）。
func Add(dir string, paths ...string) error {
	return Run(dir, append([]string{"add"}, paths...)...)
}

// Commit 在 dir 仓库中以指定 message 提交暂存区（git commit -m <message>）。
func Commit(dir string, message string) error {
	return Run(dir, "commit", "-m", message)
}

// Push 把指定 ref 推送到 remote，stdout/stderr 透传给当前终端。
// 参数：
//   - dir:    仓库工作目录（空串表示当前进程工作目录）
//   - remote: 目标 remote 名（如 "origin"）
//   - ref:    待推送的 ref（分支名 "master" 或 tag "v1.0"）；空串表示推送当前分支
//   - force:  是否强制推送（--force-with-lease，比 --force 更安全）
func Push(dir string, remote string, ref string, force bool) error {
	args := []string{"push"}
	if force {
		args = append(args, "--force-with-lease")
	}
	args = append(args, remote)
	if ref != "" {
		args = append(args, ref)
	}
	return Run(dir, args...)
}

// FindGitRoot 从 dir 开始向上查找，返回最先出现 .git(文件或目录均可) 的目录。
//
// 与 git 自身的向上查找语义一致：能识别普通仓库的 .git 目录，也能识别 worktree /
// submodule 场景下的 .git 文件。一路查到根目录都未命中则 ok=false。
func FindGitRoot(dir string) (root string, ok bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir { // 已到根目录
			return "", false
		}
		dir = parent
	}
}
