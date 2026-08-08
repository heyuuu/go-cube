package server

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `server`
func NewCommand(a *app.App) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "server",
		Short: `run the server`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Server().Start(fmt.Sprintf(":%d", port))
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")

	// children
	cmd.AddCommand(newOpenapiCmd(a))

	return cmd
}
