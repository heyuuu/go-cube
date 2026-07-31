package opener

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/util/pathkit"
)

// RootCmd 是 `cube diff` 命令入口。
var openerDiffCmd = &easycobra.Command{
	Use:   "diff <path1> <path2> [:-o|--opener= 对比工具名]",
	Short: "用对比工具(opener)对比两个路径（同为 dir 或同为 file）",
	Args:  cobra.ExactArgs(2),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var openerName string
		cmd.Flags().StringVarP(&openerName, "opener", "o", "", "对比工具(opener)名")

		return func(args []string) error {
			// 1. 绝对化两个路径
			abs1, err := pathkit.ResolvePath(args[0])
			if err != nil {
				return err
			}
			abs2, err := pathkit.ResolvePath(args[1])
			if err != nil {
				return err
			}
			// 2. 判断类型（不存在则报错）
			pt1, err := detectPathType(abs1)
			if err != nil {
				return err
			}
			pt2, err := detectPathType(abs2)
			if err != nil {
				return err
			}
			// 3. 校验类型一致
			if pt1 != pt2 {
				return fmt.Errorf("两个路径类型不一致：%s 是 %s，%s 是 %s", args[0], pt1, args[1], pt2)
			}
			// 4. 按 type 选 diff role
			role := pickDiffRole(pt1)
			// 5. 选 opener
			service := app.Default().OpenerService()
			pick, err := pickOpener(service, role, openerName)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			// 6. 对比
			if err := pick.Open(abs1, abs2); err != nil {
				return fmt.Errorf("对比失败: %w", err)
			}
			return nil
		}
	},
}

// pickDiffRole 由路径类型推导 diff role：dir→RoleDiffDir，file→RoleDiffFile。
func pickDiffRole(pt PathType) opener.Role {
	if pt == TypeDir {
		return opener.RoleDiffDir
	}
	return opener.RoleDiffFile
}
