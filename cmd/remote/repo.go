package remote

import (
	"fmt"
	"github.com/heyuuu/go-cube/cmd/alfred"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
)

var RepoCmd = &easycobra.Command[any]{
	Use:     "remote",
	Aliases: []string{"repo"},
}

func init() {
	easycobra.AddCommand(RepoCmd, repoListCmd)
}

// cmd `app list`
var repoListCmd = &easycobra.Command[any]{
	Use:   "list-hub",
	Short: "列出可用 repo-hub 列表",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		service := app.Default().RepoService()

		hubs := service.Hubs()
		showHubs(hubs)
	},
}

func showHubs(hubs []*entities.Hub) {
	if alfred.IsAlfred {
		alfred.PrintResultFunc(hubs, func(item *entities.Hub) alfred.Item {
			return alfred.Item{
				Title:    item.Name(),
				SubTitle: item.Host(),
				Arg:      item.Name(),
			}
		})
	} else {
		header := []string{
			fmt.Sprintf("Hub(%d)", len(hubs)),
			"Path",
		}
		console.PrintTableFunc(hubs, header, func(hub *entities.Hub) []string {
			return []string{
				hub.Name(),
				hub.Host(),
			}
		})
	}
}
