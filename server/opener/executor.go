package opener

import (
	"os"
	"os/exec"
)

// Executor 抽象「启动子进程」副作用，便于测试注入 fake（断言组装的命令而不真正执行）。
type Executor interface {
	Run(bin string, args ...string) error
}

// osExec 是 Executor 的默认实现：启动子进程并把 stdout/stderr 接到当前终端。
// opener 多为 GUI/编辑器，不接管子进程 stdio 时用户看不到错误，故固定接管。
type osExec struct{}

func (osExec) Run(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// NewDefaultExecutor 返回走 os/exec 的默认 Executor。
func NewDefaultExecutor() Executor { return osExec{} }
