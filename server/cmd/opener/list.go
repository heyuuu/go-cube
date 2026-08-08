package opener

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/slicekit"
	"cube/util/tui"
)

// cmd `opener list`
func newListCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [query]",
		Short: "列出可用 Opener 列表(支持模糊搜索)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			if len(args) > 0 {
				query = args[0]
			}

			service := a.OpenerService()
			apps := service.SearchAll(query)
			showOpeners(apps)
			return nil
		},
	}
	return cmd
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
