package serve

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Fork 后台启动一个 server 子进程：fork 自身跑 `cube server start`（前台模式），
// 子进程 setsid 脱离终端、stdio 丢弃，父进程 fork 完即返回。
//
// 子进程不带 --detach（否则会无限递归 fork），跑的就是普通前台 start。
// 进程的探活/关停由 HTTP API 承担，本函数只负责 fork，不持久化 pid。
//
// 返回子进程 pid（仅供日志）。
func Fork(port int) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("解析自身可执行文件路径失败: %w", err)
	}

	cmd := exec.Command(exe, "server", "start", "-p", strconv.Itoa(port))
	// Setsid 让子进程新建会话、脱离父进程的控制终端：父进程退出时不会向子进程
	// 传播 SIGHUP，子进程也不会因共享终端 fd 被父进程的退出拖死。
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// stdio 全 nil：接 /dev/null。子进程自己调 logger.Init 写 slog 到 cube.log，
	// stdio 没有保留价值（不重定向到日志文件，那会和 slog 交错写花）。

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("fork server 子进程失败: %w", err)
	}

	pid := cmd.Process.Pid
	// Release 让 Go runtime 放弃跟踪该 pid，回收由 init 完成。
	_ = cmd.Process.Release()

	slog.Info("serve: forked server subprocess", "pid", pid, "port", port)
	return pid, nil
}
