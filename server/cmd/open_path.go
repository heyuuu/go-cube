package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
)

func newOpenPathCmd(a *app.App) *cobra.Command {
	var openerName string
	cmd := &cobra.Command{
		Use:   "open-path <path> [-o|--opener=打开工具名]",
		Short: "用 opener 打开路径（按 dir/file 自动匹配 role）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查路径
			path, isDir, err := checkOpenPath(args[0])
			if err != nil {
				return err
			}

			// 判断 role 类型
			var role opener.Role
			if isDir {
				role = opener.RoleOpenDir
			} else {
				role = opener.RoleOpenFile
			}

			// 选 opener
			pick, err := pickOpener(a.OpenerService(), role, openerName)
			if err != nil {
				return err
			}

			// 打开
			if err := pick.Open(path); err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名")
	return cmd
}
