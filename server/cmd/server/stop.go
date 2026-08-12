package server

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
)

// newStopCmd `cube server stop` —— 触发后台 server 平滑关闭（POST /api/system/shutdown）。
func newStopCmd(a *app.App) *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "停止后台 server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stopped, err := serve.Stop(port)
			if err != nil {
				return err
			}
			if stopped {
				fmt.Println("server 已停止")
			} else {
				fmt.Println("server 未在运行")
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", DefaultPort, "server port")
	return cmd
}
