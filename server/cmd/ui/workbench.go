package ui

import (
	"github.com/spf13/cobra"

	"cube/app"
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
			return openPage(a, "workbench", args[0])
		},
	}
	return cmd
}
