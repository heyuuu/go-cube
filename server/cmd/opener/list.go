package opener

import (
	"fmt"

	"cube/app"
	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/opener"
	"cube/util/slicekit"
)

// cmd `opener list`
var listCmd = &easycobra.Command{
	Use:   "list [query]",
	Short: "列出可用 Opener 列表(支持模糊搜索)",
	Run: func(args []string) error {
		var query string
		if len(args) > 0 {
			query = args[0]
		}

		service := app.Default().OpenerService()
		apps := service.SearchAll(query)
		showOpeners(apps)
		return nil
	},
}

func showOpeners(apps []*opener.Opener) {
	tui.PrintTable(
		[]string{
			fmt.Sprintf("Opener(%d)", len(apps)),
			"Cmd",
			"Roles",
		},
		slicekit.Map(apps, func(app *opener.Opener) []string {
			return []string{
				app.Name(),
				app.CmdString(),
				app.RolesString(),
			}
		}),
	)
}
