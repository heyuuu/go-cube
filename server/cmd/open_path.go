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
		Long: `用 opener 打开任意路径，不限于已收录的项目。

按路径类型自动选择 role（目录 → open-dir，文件 → open-file），
再从声明了该 role 的 opener 中挑选：-o 精确指定名称；
未指定时模糊匹配，命中多个则进入交互选择。`,
		Args: cobra.ExactArgs(1),
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
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 支持模糊搜索")
	return cmd
}
