package git

// 本包是 cube 唯一的 git 访问层，读 / 写全部走系统 git 子进程。
//
// 为什么统一用系统 git（历史上读操作曾用 go-git 库）：
//   - 行为与用户日常 git 完全一致：全局忽略链（core.excludesFile / XDG）、
//     凭据助手、SSH agent、hooks 等本地生态原生复用；
//   - 读操作解析机器可读输出（porcelain / for-each-ref --format），性能依赖
//     git 自身 index 缓存，大仓库 status 毫秒级；go-git 需全量读文件算 SHA。
//
// 分工：写操作（Clone / Push / Commit 等，stdio 透传给人看）在本文件；
// 读操作（Branches / Remotes / StatusFiles 等，stdout 捕获解析）见 read.go，
// 后者注入稳定环境（LC_ALL / core.quotePath 等）保证输出可解析、不受用户配置影响。
//
// 此外，与具体 git 实现无关的辅助能力也归在本包：FindGitRoot（按 .git 探测仓库根）、
// ParseRepoUrl（解析 SSH/HTTPS 仓库地址）等，只与 git 的概念/约定相关。

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

// Pull 快进合并 remote 的指定分支到当前分支（git pull --ff-only remote ref）。
// 仅快进：本地与远端分叉时 git 报错退出，不产生 merge commit、不动工作区。
func Pull(dir string, remote string, ref string) error {
	return Run(dir, "pull", "--ff-only", remote, ref)
}

// FetchIntoBranch 把 remote 上的指定分支快进更新到本地同名分支
// （git fetch remote branch:branch）。
//
// 用于批量更新非当前分支（当前分支 git 不允许 fetch 直接更新，须走 Pull）。
// 仅快进：本地有领先提交时 git 拒绝写入，不会覆盖本地工作。
func FetchIntoBranch(dir string, remote string, branch string) error {
	return Run(dir, "fetch", remote, branch+":"+branch)
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
