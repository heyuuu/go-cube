// Package debug 存放内部调试 / 测试用命令，供开发期验证组件、调试输出。
//
// 这里的命令不带任何业务语义，随时可以增删改动，不影响正式功能。
// 典型用途：`cube debug tui` 逐一体验 tui 包的交互与渲染组件，
// 便于在调整样式或新增封装时快速看到实际效果。
package debug

import "cube/cmd/util/easycobra"

// RootCmd 是 `cube debug`（别名 `cube dbg`）命令组入口，纯分发。
var RootCmd = &easycobra.Command{
	Use:     "debug",
	Aliases: []string{"dbg"},
	Hidden:  true,
	Short:   "内部调试命令（开发期测试用）",
	Children: []*easycobra.Command{
		tuiCmd,
	},
}
