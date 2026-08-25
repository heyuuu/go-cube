// Package web 提供 `cube web` 命令族：用浏览器打开 cube Web UI 的各页面。
//
//	cube web            # 打开首页
//	cube web md <path>  # 以 Web 方式打开 markdown 文件
//
// server 不在跑时报错退出（不做 lazy 拉起，遵循「显式 start」的 server 管理模式）。
package web

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
)

// NewCmd 构建 `cube web` 父命令及其子命令。
func NewCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "打开 cube Web UI（首页 / md 页）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWeb(a)
		},
	}

	cmd.AddCommand(newMdCmd(a))

	return cmd
}

// runWeb 打开 Web UI 首页。
func runWeb(a *app.App) error {
	if st, _ := serve.Status(a.Server().Port()); !st.Running {
		return errors.New("server 未运行，请先执行: cube server start")
	}
	openInBrowser(a.Server().ServerURL())
	return nil
}

// openInBrowser 用系统 open 打开 URL。失败不阻断：打出 URL 让用户手动访问。
func openInBrowser(pageURL string) {
	if err := exec.Command("open", pageURL).Run(); err != nil {
		fmt.Printf("自动打开浏览器失败，请手动访问：%s\n", pageURL)
	} else {
		fmt.Printf("%s\n", pageURL)
	}
}
