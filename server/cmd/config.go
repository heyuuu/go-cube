package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/config"
	"cube/version"
)

func newConfigCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "show config",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cube " + version.Version)
			fmt.Println("config path: " + config.Path())
			return nil
		},
	}
	return cmd
}
