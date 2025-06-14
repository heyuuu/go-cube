package cmd

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/spf13/cobra"
)

var repoCmd = initCmd(cmdOpts[any]{
	Use: "repo",
})

// cmd `app list`
var repoListCmd = initCmd(cmdOpts[any]{
	Root:  repoCmd,
	Use:   "list-hub",
	Short: "列出可用 repo-hub 列表",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		service := app.Default().RepoService()

		hubs := service.Hubs()
		showHubs(hubs)
	},
})

func showHubs(hubs []*entities.Hub) {
	if isAlfred {
		alfredSearchResultFunc(hubs, (*entities.Hub).Name, (*entities.Hub).Host, (*entities.Hub).Name)
	} else {
		header := []string{
			fmt.Sprintf("Hub(%d)", len(hubs)),
			"Path",
		}
		printTableFunc(hubs, header, (*entities.Hub).Name, (*entities.Hub).Host)
	}
}
