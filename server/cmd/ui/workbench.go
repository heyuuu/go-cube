package ui

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
	"cube/util/pathkit"
)

// newWorkbenchCmd `cube ui workbench <path>` —— 打开项目的工作台页面（与前端路由 /workbench 同名对齐）。
//
// 这是「打开工作台」这类 opener 的落地形态：opener 配置成 exec 命令
// `["cube", "ui", "workbench", "$0"]` 即可，URL 拼接（端口/路由/转义）收敛在本命令。
func newWorkbenchCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workbench <path>",
		Short: "打开工作台（workbench）页面",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkbench(a, args[0])
		},
	}
	return cmd
}

func runWorkbench(a *app.App, rawPath string) error {
	// 路径解析同 md：支持 ~ 与相对路径，统一转绝对路径（web API 只收绝对路径）
	if rawPath[0] != '/' && rawPath[0] != '~' && rawPath[0] != '.' {
		rawPath = "./" + rawPath
	}
	absPath, err := pathkit.AbsPath(rawPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("路径不存在: %s", absPath)
	}

	if st := serve.Status(a.Server().Port()); !st.Running {
		return errors.New("server 未运行，请先执行: cube server start")
	}

	pageURL := fmt.Sprintf("%sworkbench?path=%s", a.Server().ServerURL(), url.QueryEscape(absPath))
	openInBrowser(pageURL)
	return nil
}
