// Package dev 存放内部调试 / 测试用命令，供开发期验证组件、调试输出。
//
// 这里的命令不带任何业务语义，随时可以增删改动，不影响正式功能。
// 典型用途：`cube dev tui` 逐一体验 tui 包的交互与渲染组件，
// 便于在调整样式或新增封装时快速看到实际效果。
package dev

import (
	"github.com/spf13/cobra"

	"cube/app"
)

// `cube dev`

func NewCommand(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dev",
		Hidden: true,
		Short:  "内部调试命令（开发期测试用）",
	}
	cmd.AddCommand(newTuiCmd(a))
	return cmd
}
