package ui

import (
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
)

// cmd `ui` —— 启动 server 并用默认浏览器打开页面。
// 等价于 `cube server` + 自动 open http://localhost:port/。
var RootCmd = &easycobra.Command{
	Use:   "ui",
	Short: `启动 server 并打开浏览器`,
	Args:  cobra.NoArgs,
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var port int
		var noOpen bool
		cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")
		cmd.Flags().BoolVar(&noOpen, "no-open", false, "不自动打开浏览器")

		return func(args []string) error {
			url := fmt.Sprintf("http://localhost:%d/", port)

			// 异步：等 server 监听后开浏览器（避免浏览器先打开连不上）
			if !noOpen {
				go openBrowserWhenReady(url, port)
			}

			server := app.Default().Server()
			slog.Info("cube ui", "url", url)
			return server.Start(fmt.Sprintf(":%d", port))
		}
	},
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
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{url}
	case "linux":
		return "xdg-open", []string{url}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", url}
	}
	// 兜底：macOS 风格
	return "open", []string{url}
}
