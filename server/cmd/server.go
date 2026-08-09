package cmd

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/web"
)

// cmd `server` —— 启动 server
func newServerCmd(a *app.App) *cobra.Command {
	var port int
	var open bool
	cmd := &cobra.Command{
		Use:   "server",
		Short: `启动 server`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return startServer(a.Server(), port, open)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")
	cmd.Flags().BoolVarP(&open, "open", "O", false, "open browser")

	return cmd
}

func startServer(server *web.Server, port int, open bool) error {
	url := fmt.Sprintf("http://localhost:%d/", port)
	slog.Info("Server starting", "url", url)

	if open {
		// 异步：等 server 监听后开浏览器（避免浏览器先打开连不上）
		go openBrowserWhenReady(url, port)
	}

	return server.Start(fmt.Sprintf(":%d", port))
}

// openBrowserWhenReady 轮询端口直到 server 监听，然后用默认浏览器打开 url。
// 超时（10s）放弃（不阻断 server 本身）。
func openBrowserWhenReady(url string, port int) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	// 稍等一拍，让 huma 路由就绪
	time.Sleep(150 * time.Millisecond)

	if err := openBrowser(url); err != nil {
		slog.Warn("打开浏览器失败，请手动访问", "url", url, "err", err)
	}
}

// openBrowser 用当前平台的默认浏览器打开 url（fire-and-forget，不接管 stdio）。
func openBrowser(url string) error {
	bin, args := openCmd(url)
	cmd := exec.Command(bin, args...)
	// Stdin/Stdout/Stderr 留 nil：浏览器进程脱离 cube 独立运行
	return cmd.Start()
}

// openCmd 按平台返回「打开 URL」的命令。主平台 macOS 用 open，其余平台尽力覆盖。
func openCmd(url string) (string, []string) {
	// 兜底：macOS 风格
	return "open", []string{url}
}
