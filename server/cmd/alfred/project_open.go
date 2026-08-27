package alfred

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
)

// cmd `alfred project-open`
func newProjectOpenCmd(a *app.App) *cobra.Command {
	var openerName string

	cmd := &cobra.Command{
		Use:   "project-open <target-path>",
		Short: "打开项目, 只支持准确目标目录绝对路径（项目根或 worktree）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := args[0]

			// 匹配项目：目标路径可能是项目根或 worktree（1032 归并到主项目）
			proj := a.ProjectService().ResolveProject(targetPath)
			if proj == nil {
				return errors.New("未找到指定项目: " + targetPath)
			}

			// 按 opener 名精确查找（区别于主命令的模糊 pickOpener）
			o := a.OpenerService().FindByName(openerName)
			if o == nil {
				return errors.New("未找到指定 opener: " + openerName)
			}

			// 打开目标目录
			err := o.Open(opener.RoleOpenDir, targetPath)
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			// 记录使用信号（best-effort：失败不影响打开结果）；dir 直接传目标目录
			// （等于项目根时由 RecordOpen 归一为空）
			if err := a.UsageService().RecordOpen(proj.Path(), o.Name(), targetPath); err != nil {
				slog.Warn("记录 usage 失败", "err", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 精确匹配")
	return cmd
}
