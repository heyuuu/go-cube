package project

import (
	"fmt"

	"cube/app"
	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/project"
	"cube/util/slicekit"
)

// cmd `project clone-rules`
var cloneRulesCmd = &easycobra.Command{
	Use:   "clone-rules",
	Short: "列出 clone 规则",
	Run: func(args []string) error {
		service := app.Default().ProjectService()
		rules := service.CloneRules()

		// 显示列表
		tui.PrintTable(
			[]string{
				fmt.Sprintf("RepoHost(%d)", len(rules)),
				"RepoPrefix",
				"LocalPath",
			},
			slicekit.Map(rules, func(r project.CloneRule) []string {
				return []string{
					r.RepoHost,
					r.RepoPrefix,
					r.LocalPath,
				}
			}),
		)
		return nil
	},
}
