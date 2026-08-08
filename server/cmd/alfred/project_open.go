package alfred

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/util/easycobra"
)

// cmd `alfred project-open`
var projectOpenCmd = &easycobra.Command{
	Use:   "project-open <project-name>",
	Short: "打开项目, 只支持准确项目绝对路径",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var openerName string
		cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开项目的 Opener")

		// run
		return func(args []string) error {
			projectName := args[0]

			// history: 记录打开项目的程序
			app.Default().HistoryService().AddProjectOpenLog(projectName, openerName, true)

			// 匹配项目
			projService := app.Default().ProjectService()
			proj := projService.FindByName(projectName)
			if proj == nil {
				return errors.New("未找到指定项目: " + projectName)
			}

			// 获取打开项目的app
			openerService := app.Default().OpenerService()
			opener := openerService.FindByName(openerName)
			if opener == nil {
				return errors.New("未找到指定 opener: " + openerName)
			}

			// 打开项目
			err := opener.Open(proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		}
	},
}
