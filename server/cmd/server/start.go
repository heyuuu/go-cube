package server

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
	"cube/web"
)

// DefaultPort server 默认端口。
// 允许起多个不同端口的 server，端口由 -p 指定，不传则用此默认值。
const DefaultPort = 8080

// newStartCmd `cube server start` —— 启动 server。
//
// 默认前台（开发/调试用，Ctrl+C 退）；--detach 后台 fork 脱终端。
func newStartCmd(a *app.App) *cobra.Command {
	return newStartCmdEx(a, "start", "前台启动 server（Ctrl+C 退出）")
}

func newStartCmdEx(a *app.App, use string, short string) *cobra.Command {
	var port int
	var detach bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if detach {
				return startDetached(port)
			}
			return startServer(a.Server(), port)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", DefaultPort, "server port")
	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "后台启动（fork 脱终端，不占 stdout）")
	return cmd
}

// startDetached 后台 fork 一个 server 子进程（参 serve.Fork）。
func startDetached(port int) error {
	pid, err := serve.Fork(port)
	if err != nil {
		return err
	}
	fmt.Printf("server 后台启动中（pid=%d）\n", pid)
	fmt.Printf("  访问地址：%s\n", serverURL(port))
	return nil
}

func startServer(server *web.Server, port int) error {
	fmt.Printf("server 启动中\n")
	fmt.Printf("  访问地址：%s\n", serverURL(port))
	slog.Info("server starting", "port", port)
	return server.Start(fmt.Sprintf(":%d", port))
}

// serverURL 拼出 server 的访问地址。
func serverURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/", port)
}
