package workspace

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
)

var WorkspaceCmd = &easycobra.Command{
	Use:     "workspace",
	Aliases: []string{"ws"},
}

func init() {
	WorkspaceCmd.AddCommand(workspaceListCmd)
}

// cmd `workspace list`
var workspaceListCmd = &easycobra.Command{
	Use: "list",
	Run: func(cmd *cobra.Command, args []string) {
		service := app.Default().ProjectService()
		for _, ws := range service.Workspaces() {
			fmt.Println(ws.Name())
		}
	},
}
