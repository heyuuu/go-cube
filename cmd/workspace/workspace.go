package workspace

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
)

var WorkspaceCmd = &easycobra.Command[any]{
	Use:     "workspace",
	Aliases: []string{"ws"},
}

func init() {
	easycobra.AddCommand(WorkspaceCmd, workspaceListCmd)
}

// cmd `workspace list`
var workspaceListCmd = &easycobra.Command[any]{
	Use: "list",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		service := app.Default().ProjectService()
		for _, ws := range service.Workspaces() {
			fmt.Println(ws.Name())
		}
	},
}
