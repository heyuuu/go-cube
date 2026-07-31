package opener

import (
	"fmt"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/util/slicekit"
)

var RootCmd = &easycobra.Command{
	Use: "opener",
	Children: []*easycobra.Command{
		openerListCmd,
	},
}

var openerListCmd = &easycobra.Command{
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
