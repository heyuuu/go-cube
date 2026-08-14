package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
)

// cmd `cube open`
func newOpenCmd(a *app.App) *cobra.Command {
	var openerName string
	cmd := &cobra.Command{
		Use:   "open [query] [-o|--opener=打开工具名]",
		Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
		Long: `用指定 app(opener) 打开一个已收录的项目目录。

query 支持项目名和项目列表模糊搜索，具体规则同 info 命令。
--opener 为 opener 名称，支持模糊搜索。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := getArg(args, 0)

			// 匹配项目
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}

			// 选 opener
			openApp, err := pickOpener(a.OpenerService(), opener.RoleOpenDir, openerName)
			if err != nil {
				return err
			}

			// 打开项目
			err = openApp.Open(proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 支持模糊搜索")
	return cmd
}
