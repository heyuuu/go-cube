package git

// 读执行核：全部读操作（refs.go / status.go / tree.go / diff.go / log.go /
// worktree.go）共用 runOut 走系统 git 子进程，只解析机器可读输出，不解析任何
// 面向人的文案（stderr 随 locale 翻译，不可依赖）。
//
// 输出稳定性（规避用户本地 git 配置差异，见 runOut 与各命令的显式 flag）：
//   - LC_ALL=C、-c core.quotePath=false、GIT_PAGER=cat 统一注入；
//   - status 显式带 --untracked-files=normal，覆盖 status.showUntrackedFiles=no。
//
// 错误处理约定（全包一致，上层缓存层依赖）：
//   - 非 git 目录、缺失 remote/分支等「业务上可接受的空值」场景：返回零值 + nil；
//   - 真实读取错误（损坏的 .git、IO 异常等）：返回零值 + error，由调用方决定是否记录。

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// readCmdLogEnabled 控制「git 读命令」逐条日志的开关。
// gitcache 全量采集时一次会跑几百条 git 子进程，逐条 Debug 日志会淹没其他输出；
// 默认静默，仅设置 CUBE_GITCACHE_TRACE（任意非空值）时打印，级别保持 Debug。
var readCmdLogEnabled = sync.OnceValue(func() bool {
	return os.Getenv("CUBE_GITCACHE_TRACE") != ""
})

// runOut 在 dir 下执行 git 读命令并捕获 stdout（不透传终端）。
// 与 Run 的差异：输出面向程序解析而非人，因此注入与用户配置无关的稳定环境：
//   - LC_ALL=C：统一 locale（porcelain 格式本身不受 locale 影响，防御性兜底）；
//   - GIT_PAGER=cat：禁用分页器，避免极端配置下进程等待交互翻页挂起；
//   - --no-optional-locks：后台读不碰 index.lock，不与用户正在进行的 git 操作抢锁；
//   - -c core.quotePath=false：非 ASCII 路径（如中文文件名）不转义成八进制串。
//
// stderr 不做解析（文案随 locale 翻译，不可依赖），只作为错误信息附带给日志。
func runOut(dir string, args ...string) (string, error) {
	args = append([]string{"--no-optional-locks", "-c", "core.quotePath=false"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_PAGER=cat")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if readCmdLogEnabled() {
		slog.Debug("git 读命令", "cmd", cmd.String())
	}
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s 执行失败: %w；stderr: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// isGitRepo 判断 path 自身是否为 git 仓库根（存在 .git 文件或目录，worktree 的
// .git 文件也算）。bare 仓库（目录本身即 gitdir，无 .git）按非仓库降级——
// cube 收录的项目必然是普通工作区副本。
//
// 读命令靠它把「非仓库」从子进程错误里区分出来：git 的报错文案随 locale 变化，
// 不能解析 stderr 识别，而一次 os.Stat 的预判开销可以忽略。
func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// firstLine 取输出第一行并去首尾空白，空输出返回空串。
func firstLine(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	return strings.TrimSpace(line)
}
