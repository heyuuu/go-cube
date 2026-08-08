// Package ugly 存放个人定制 / 临时 hack 命令。
//
// 这里的代码不追求通用性，允许写死、允许丑陋（ugly）。
// 命名 ugly 类比其它语言里的 unsafe：
// 每次敲 `cube ugly xxx` 都是一次自我提醒——「这是不干净的、写死的，
// 一旦时机成熟应将其通用化后迁移出本包，或直接删除」。
// 也提供 `ug` 别名便于日常快速输入。
//
// 本包只做分发与编排：通用能力（git/tui/project 等封装）一律复用既有包，
// 不在此处重复造轮子，也不污染通用包。
package ugly

import "cube/cmd/util/easycobra"

// RootCmd 是 `cube ugly`（别名 `cube ug`）命令组入口，纯分发。
//
// 在此追加定制子命令；每条都应有计划被通用化或移除。
var RootCmd = &easycobra.Command{
	Use:      "ugly",
	Aliases:  []string{"ug"},
	Short:    "ugly hacks: 临时定制命令，应尽早通用化",
	Children: []*easycobra.Command{},
}
