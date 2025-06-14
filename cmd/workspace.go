package cmd

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/project"
	"github.com/spf13/cobra"
)

var workspaceCmd = initCmd(cmdOpts[any]{
	Use:     "workspace",
	Aliases: []string{"ws"},
})

// cmd `workspace list`
var workspaceListCmd = initCmd(cmdOpts[any]{
	Root: workspaceCmd,
	Use:  "list",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		pm := project.DefaultManager()
		for _, ws := range pm.Workspaces() {
			fmt.Println(ws.Name())
		}
	},
})
