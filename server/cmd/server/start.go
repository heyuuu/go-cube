package server

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
	"cube/version"
)

// newStartCmd `cube server start` —— 启动 server。
//
// 默认前台（开发/调试用，Ctrl+C 退）；--detach 后台 fork 脱终端。
func newStartCmd(a *app.App) *cobra.Command {
	return newStartCmdEx(a, "start", "前台启动 server（Ctrl+C 退出）")
}

func newStartCmdEx(a *app.App, use string, short string) *cobra.Command {
	var detach bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detach {
				return startDetached(a)
			}
			return startServer(a)
		},
	}
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "后台启动（fork 脱终端，不占 stdout）")
	return cmd
}

// startDetached 后台 fork 一个 server 子进程（参 serve.Fork）。
func startDetached(a *app.App) error {
	pid, err := serve.Fork()
	if err != nil {
		return err
	}
	fmt.Printf("cube version: %s\n", version.VersionInfo())
	fmt.Printf("server 后台启动中（pid=%d）\n", pid)
	fmt.Printf("  访问地址：%s\n", a.Server().ServerURL())
	return nil
}

func startServer(a *app.App) error {
	// 常驻 server 启动后台任务（项目视图定时刷新等）；server 退出（Start 返回）时停止。
	a.StartBackgroundJobs()
	defer a.StopBackgroundJobs()

	fmt.Printf("cube version: %s\n", version.VersionInfo())
	fmt.Printf("server 启动中\n")
	fmt.Printf("  访问地址：%s\n", a.Server().ServerURL())
	slog.Info("server 启动中", "url", a.Server().ServerURL())
	return a.Server().Start()
}
