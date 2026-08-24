package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/version"
)

func newVersionCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "show version",
		Long:  `显示 cube 当前版本号。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cube " + version.VersionInfo())
			return nil
		},
	}
	return cmd
}
