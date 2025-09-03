package remote

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
)

var RemoteCmd = &easycobra.Command{
	Use: "remote",
}

func init() {
	RemoteCmd.AddCommand(remoteListCmd)
}

// cmd `remote list`
var remoteListCmd = &easycobra.Command{
	Use:   "list",
	Short: "列出可用远端仓库列表",
	Run: func(cmd *cobra.Command, args []string) {
		service := app.Default().RemoteService()
		remotes := service.Remotes()
		showRemotes(remotes)
	},
}

func showRemotes(remotes []*entities.Remote) {
	header := []string{
		fmt.Sprintf("Remote(%d)", len(remotes)),
		"Path",
	}
	console.PrintTableFunc(remotes, header, func(hub *entities.Remote) []string {
		return []string{
			hub.Name(),
			hub.Host(),
		}
	})
}
