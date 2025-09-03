package remote

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
)

var RemoteCmd = &easycobra.Command[any]{
	Use: "remote",
}

func init() {
	easycobra.AddCommand(RemoteCmd, remoteListCmd)
}

// cmd `remote list`
var remoteListCmd = &easycobra.Command[any]{
	Use:   "list",
	Short: "列出可用远端仓库列表",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		service := app.Default().RemoteService()
		remotes := service.Remotes()
		showRemotes(remotes)
	},
}

func showRemotes(hubs []*entities.Remote) {
	header := []string{
		fmt.Sprintf("Remote(%d)", len(hubs)),
		"Path",
	}
	console.PrintTableFunc(hubs, header, func(hub *entities.Remote) []string {
		return []string{
			hub.Name(),
			hub.Host(),
		}
	})
}
