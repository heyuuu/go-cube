package alfred

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
)

// cmd `alfred project-open`
func newProjectOpenCmd(a *app.App) *cobra.Command {
	var openerName string

	cmd := &cobra.Command{
		Use:   "project-open <project-name>",
		Short: "打开项目, 只支持准确项目绝对路径",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectName := args[0]

			// history: 记录打开项目的程序
			a.HistoryService().AddProjectOpenLog(projectName, openerName, true)

			// 匹配项目
			proj := a.ProjectService().FindByName(projectName)
			if proj == nil {
				return errors.New("未找到指定项目: " + projectName)
			}

			// 按 opener 名精确查找（区别于主命令的模糊 pickOpener）
			o := a.OpenerService().FindByName(openerName)
			if o == nil {
				return errors.New("未找到指定 opener: " + openerName)
			}

			// 打开项目
			err := o.Open(opener.RoleOpenDir, proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 精确匹配")
	return cmd
}
