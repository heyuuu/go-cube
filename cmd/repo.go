package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var repoCmd = initCmd(cmdOpts[any]{
	Use: "repo",
})

// cmd `app list`
var repoListCmd = initCmd(cmdOpts[any]{
	Root: repoCmd,
	Use:  "list",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		fmt.Println("repo list called")
	},
})
