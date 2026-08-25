package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/slicekit"
	"cube/util/tui"
)

func newOpenersCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "openers [query]",
		Short: "列出可用 Opener 列表(支持模糊搜索)",
		Long: `显示可用 opener 列表：名称、执行命令及声明的 roles。

query 按 opener 名称模糊搜索，不传时显示全部。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			if len(args) > 0 {
				query = args[0]
			}

			service := a.OpenerService()
			openers := service.SearchAll(query)
			showOpeners(openers)
			return nil
		},
	}
	return cmd
}

func showOpeners(list []opener.Opener) {
	tui.PrintTable(
		[]string{
			fmt.Sprintf("Opener(%d)", len(list)),
			"Summary",
			"Roles",
		},
		slicekit.Map(list, func(o opener.Opener) []string {
			return []string{
				o.Name(),
				o.Summary(),
				opener.RolesString(o.Roles()),
			}
		}),
	)
}
