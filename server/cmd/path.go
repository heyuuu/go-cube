package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube path`
func newPathCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path [query]",
		Short: "输出项目路径。选择流程同 open（项目匹配 + 打开目标），只输出不打开",
		Long: `输出选中项目的打开目标路径（主目录 / worktree / workspace）。

选择流程与 open 命令一致：query 支持项目名和模糊搜索（同 info 命令），
多打开目标时交互选择。stdout 只输出最终路径一行，
配合 shell 包装函数实现跳转：cd "$(cube path)"。
取消选择或非 TTY 多目标时以非零退出码结束，不输出路径。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := getArg(args, 0)

			// 匹配项目（与 open 一致）
			proj, err := pickProject(a.ProjectService(), query)
			if err != nil {
				return err
			}

			// 选打开目标（与 open 一致：worktree / workspace 归并为项目打开目标）
			target, err := pickOpenTarget(a.ProjectService(), proj)
			if err != nil {
				return err
			}

			fmt.Println(target)
			return nil
		},
	}
	return cmd
}
