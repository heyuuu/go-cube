package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/tui"
)

// newDiffCmd 是 `cube diff` 命令入口。
func newDiffCmd(a *app.App) *cobra.Command {
	var openerName string
	cmd := &cobra.Command{
		Use:   "diff <path1> <path2> [-o|--opener=打开工具名]",
		Short: "用对比工具(opener)对比两个路径（同为 dir 或同为 file）",
		Long: `用对比工具(opener)对比两个路径，路径须真实存在且类型一致
（同为目录或同为文件）。

按路径类型自动选择 role（目录 → diff-dir，文件 → diff-file），
再从声明了该 role 的 opener 中挑选：-o 精确指定名称；
未指定时模糊匹配，命中多个则进入交互选择。`,
		Args: cobra.ExactArgs(2),
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
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 支持模糊搜索")
	return cmd
}
