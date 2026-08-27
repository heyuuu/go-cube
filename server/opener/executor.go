package opener

import (
	"fmt"
	"log/slog"
	"os/exec"
	"syscall"
	"time"
)

// Executor 抽象「启动子进程」副作用，便于测试注入 fake（断言组装的命令而不真正执行）。
type Executor interface {
	Run(bin string, args ...string) error
}

// earlyExitWindow 启动后的观察窗口：窗口内子进程就退出视为启动失败
// （命令不存在之外的大多数错误——参数错、app 拒绝启动——都发生在这一瞬间），
// 错误还能返回给调用方；过了窗口才转后台放生，此后退出仅记日志。
const earlyExitWindow = 1500 * time.Millisecond

type osExec struct{}

// osExec 是 Executor 的默认实现：后台放生式启动子进程。
//   - Setpgid 让子进程自立进程组：终端 Ctrl+C 的组播 SIGINT、cube 进程的
//     生死都不再影响子进程（opener 多为 GUI 编辑器，冷启动慢、需长期存活）；
//   - stdio 保持 nil（接 /dev/null）：放生形态不能接随父进程消亡的 pipe
//     （否则子进程后续写输出会吃 SIGPIPE 被杀），GUI 输出对本就无观察价值；
//   - 启动失败要可感知：观察窗口内退出（非 nil Wait 结果）返回错误，
//     过窗后由 goroutine 收尸（防僵尸）并记日志。
func (osExec) Run(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 %s 失败: %w", bin, err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s 启动后立即退出: %w", bin, err)
		}
		return nil
	case <-time.After(earlyExitWindow):
		go func() {
			if err := <-done; err != nil {
				slog.Debug("opener 子进程后台退出", "bin", bin, "err", err)
			}
		}()
		return nil
	}
}

// NewDefaultExecutor 返回走 os/exec 的默认 Executor。
func NewDefaultExecutor() Executor { return osExec{} }
