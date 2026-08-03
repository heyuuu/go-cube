package project

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
)

// cmd `project open`
var openCmd = &easycobra.Command{
	Use:   "open {project : 项目名} {--app= : 打开项目的App}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var appName string
		cmd.Flags().StringVar(&appName, "app", "", "打开项目的App")

		// run
		return func(args []string) error {
			query := args[0]

			// 获取打开项目的app
			openerService := app.Default().OpenerService()
			openApp := openerService.FindByName(appName)
			if openApp == nil {
				return errors.New("未找到指定app: " + appName)
			}

			// 匹配项目
			proj := selectProject(query)
			if proj == nil {
				return nil
			}

			// 打开项目
			err := openApp.Open(proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		}
	},
}
