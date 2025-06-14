package cmd

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/repo"
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
		hubs := repo.DefaultManager().Hubs()
		showHubs(hubs)
	},
})

func showHubs(hubs []*repo.Hub) {
	if isAlfred {
		alfredSearchResultFunc(hubs, (*repo.Hub).Name, (*repo.Hub).Host, (*repo.Hub).Name)
	} else {
		header := []string{
			fmt.Sprintf("Hub(%d)", len(hubs)),
			"Path",
		}
		printTableFunc(hubs, header, (*repo.Hub).Name, (*repo.Hub).Host)
	}
}
