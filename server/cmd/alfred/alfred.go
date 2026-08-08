// Package alfred 提供 `cube alfred` 命令组，输出 Alfred Script Filter JSON，
// 供 Alfred workflow 集成（项目搜索、opener 搜索、项目打开）。
//
// 各业务子命令按「一命令一文件」组织（project_search.go / opener_search.go / project_open.go），
// 本文件只放命令组入口 RootCmd；通用的 Alfred JSON 输出 helper 在 helpers.go。
package alfred

import "cube/cmd/util/easycobra"

// RootCmd 是 `cube alfred` 命令组入口，纯分发。
var RootCmd = &easycobra.Command{
	Use: "alfred",
	Children: []*easycobra.Command{
		projectSearchCmd,
		projectOpenCmd,
		openerSearchCmd,
	},
}
