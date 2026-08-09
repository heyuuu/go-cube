package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `project open`
func newOpenCmd(a *app.App) *cobra.Command {
	var appName string
	cmd := &cobra.Command{
		Use:   "open {project : 项目名} {--app= : 打开项目的App}",
		Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			// 获取打开项目的app
			openerService := a.OpenerService()
			openApp := openerService.FindByName(appName)
			if openApp == nil {
				return errors.New("未找到指定app: " + appName)
			}

			// 匹配项目
			proj := selectProject(a.ProjectService(), query)
			if proj == nil {
				return nil
			}

			// 打开项目
			err := openApp.Open(proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&appName, "app", "", "打开项目的App")
	return cmd
}
