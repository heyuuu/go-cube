package server

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/serve"
	"cube/util/tui"
)

// newStatusCmd `cube server status` —— 探活（GET /api/system/whoami）。
func newStatusCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "查看 server 运行状态",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(a)
		},
	}
	return cmd
}

func runStatus(a *app.App) error {
	printStatus(a, serve.Status(a.Server().Port()))
	return nil
}

func printStatus(a *app.App, st serve.StatusInfo) {
	state := "未运行"
	version := "-"
	instance := "-"
	url := "-"
	if st.Running {
		state = "运行中"
		version = st.Version
		instance = st.Instance
		url = a.Server().ServerURL()
	}
	tui.PrintTable(
		[]string{"状态", "端口", "版本", "实例", "访问地址"},
		[][]string{{state, fmt.Sprintf("%d", a.Server().Port()), version, instance, url}},
	)
}
