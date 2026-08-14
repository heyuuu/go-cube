// Package server 提供 `cube server` 命令族：管理本地 web server 的生命周期。
//
// 命令族（进程管理走 HTTP API，参 docs/archived/260811-server后台常驻与HTTP管理/）：
//
//	cube server              # = cube server start（兼容现状）
//	cube server start        # 前台启动（开发/调试用）
//	cube server start -d     # 后台启动（fork 脱终端）
//	cube server stop         # 触发后台 server 平滑关闭（POST /api/system/shutdown）
//	cube server status       # 探活（GET /api/system/whoami）
package server

import (
	"github.com/spf13/cobra"

	"cube/app"
)

// NewCmd 构建 `cube server` 父命令及其子命令。
func NewCmd(a *app.App) *cobra.Command {
	cmd := newStartCmdEx(a, "server", "管理本地 web server（start / stop / status）")

	cmd.AddCommand(newStartCmd(a))
	cmd.AddCommand(newStopCmd(a))
	cmd.AddCommand(newStatusCmd(a))

	return cmd
}
