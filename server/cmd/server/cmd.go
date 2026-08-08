package server

import (
	"context"
	"fmt"
	"log/slog"

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
			// 后台监听配置文件变更，热 reload 到各 service（无需重启 server）
			go func() {
				if err := a.WatchConfig(context.Background()); err != nil {
					slog.Warn("config watcher exited", "err", err)
				}
			}()
			return a.Server().Start(fmt.Sprintf(":%d", port))
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 8080, "server port")

	// children
	cmd.AddCommand(newOpenapiCmd(a))

	return cmd
}
