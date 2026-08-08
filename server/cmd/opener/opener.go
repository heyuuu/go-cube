// Package opener 提供 `cube opener`（别名 o/open）命令组，管理打开方式：
// 列出 opener、用 opener 打开路径、用对比工具对比两路径。
//
// 各业务子命令按「一命令一文件」组织（list.go / open.go / diff.go），
// 本文件只放命令组入口 RootCmd。
package opener

import "cube/cmd/util/easycobra"

// RootCmd 是 `cube opener`（别名 o/open）命令组入口，纯分发。
var RootCmd = &easycobra.Command{
	Use:     "opener",
	Aliases: []string{"o", "open"},
	Children: []*easycobra.Command{
		listCmd,
		openCmd,
		diffCmd,
	},
}
