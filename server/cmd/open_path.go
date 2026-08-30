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
		Short: "用 opener 打开路径（按 dir/file 自动选 intent）",
		Long: `用 opener 打开任意路径，不限于已收录的项目。

按路径类型自动选择 intent（目录 → dir，文件 → file）。
不传 -o 时使用该 intent 的默认 opener；-o 不带值时交互选择；-o <name> 按名模糊匹配。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查路径
			path, isDir, err := checkOpenPath(args[0])
			if err != nil {
				return err
			}

			// 判断 intent（目录 → dir，文件 → file），不带 -o 时用默认 opener
			intent := opener.IntentFile
			role := opener.RoleOpenFile
			if isDir {
				intent, role = opener.IntentDir, opener.RoleOpenDir
			}
			pick, err := pickOpener(a.OpenerService(), intent, openerName)
			if err != nil {
				return err
			}

			// 打开
			if err := pick.Open(role, path); err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名；不带值时交互选择，缺省用默认 opener")
	cmd.Flags().Lookup("opener").NoOptDefVal = interactivePick
	return cmd
}
