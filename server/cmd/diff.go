package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/tui"
)

// RootCmd 是 `cube diff` 命令入口。
func newDiffCmd(a *app.App) *cobra.Command {
	var openerName string
	cmd := &cobra.Command{
		Use:   "diff <path1> <path2> [:-o|--opener= 对比工具名]",
		Short: "用对比工具(opener)对比两个路径（同为 dir 或同为 file）",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 检查两个路径
			path1, isDir1, err := checkOpenPath(args[0])
			if err != nil {
				return err
			}
			path2, isDir2, err := checkOpenPath(args[1])
			if err != nil {
				return err
			}

			// 判断 role 类型
			if isDir1 != isDir2 {
				return fmt.Errorf("两个路径类型不一致：%s 的 isDir=%v，%s 的 isDir=%v", args[0], isDir1, args[1], isDir2)
			}
			var role opener.Role
			if isDir1 {
				role = opener.RoleDiffDir
			} else {
				role = opener.RoleDiffFile
			}

			// 选 opener
			pick, err := pickOpener(a.OpenerService(), role, openerName)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}

			// 打开
			if err := pick.Open(path1, path2); err != nil {
				return fmt.Errorf("打开对比软件失败: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "对比工具(opener)名")
	return cmd
}
