package opener

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/opener"
	"cube/util/pathkit"
)

// RootCmd 是 `cube open` 命令入口。
var openCmd = &easycobra.Command{
	Use:   "open <path> [-o|--opener=打开工具名]",
	Short: "用 opener 打开路径（按 dir/file 自动匹配 role）",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var openerName string
		cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名")

		return func(args []string) error {
			// 1. 绝对化路径
			abs, err := pathkit.ResolvePath(args[0])
			if err != nil {
				return err
			}
			// 2. 判断类型（不存在则报错）
			pt, err := detectPathType(abs)
			if err != nil {
				return err
			}
			// 3. 按 type 选 role
			role := pickOpenRole(pt)
			// 4. 选 opener（TTY 模糊/交互，非 TTY 精确）
			service := app.Default().OpenerService()
			pick, err := pickOpener(service, role, openerName)
			if err != nil {
				if errors.Is(err, tui.ErrUserAborted) {
					return nil
				}
				return err
			}
			// 5. 打开
			if err := pick.Open(abs); err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}
			return nil
		}
	},
}

// pickOpenRole 由路径类型推导 open role：dir→RoleOpenDir，file→RoleOpenFile。
func pickOpenRole(pt PathType) opener.Role {
	if pt == TypeDir {
		return opener.RoleOpenDir
	}
	return opener.RoleOpenFile
}
