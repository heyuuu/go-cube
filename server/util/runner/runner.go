// Package runner 提供执行外部命令的工具。
//
// 用于在 cube 内调用外部程序（如 IDE、git clone、open 等），把子进程的
// stdout/stderr 直接接到当前终端，让用户能看到实时输出（进度条、错误等）。
package runner

import (
	"os"
	"os/exec"
)

// Run 执行 bin args...，stdout/stderr 透传给当前进程的终端，返回执行错误。
//
// 与 exec.Command 的区别仅在于固定接管 stdio —— 适合「让外部程序接管终端输出」的场景，
// 例如：runner.Run("code", projPath)、runner.Run("stree", projPath)。
func Run(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
