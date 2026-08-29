package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/project"
	"cube/util/tui"
)

// cmd `cube open`
func newOpenCmd(a *app.App) *cobra.Command {
	var openerName string
	cmd := &cobra.Command{
		Use:   "open [query] [-o|--opener=打开工具名]",
		Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
		Long: `用指定 opener 打开一个已收录的项目目录。

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
			role := opener.RoleOpenDir
			o, err := pickOpener(a.OpenerService(), role, openerName)
			if err != nil {
				return err
			}

			// 选打开目标（1032 worktree / 1030 workspace 归并为项目打开目标）：多目标时交互选择，单目标流程不变
			target, err := pickOpenTarget(a.ProjectService(), proj)
			if err != nil {
				return err
			}

			// 打开项目
			err = o.Open(role, target)
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			// 记录使用信号（best-effort：失败不影响打开结果）；project 恒记主项目路径，
			// dir 直接传实际打开的目标目录（等于根时由 RecordOpen 归一为空）
			if err := a.UsageService().RecordOpen(proj.Path(), o.Name(), target); err != nil {
				slog.Warn("记录 usage 失败", "err", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开工具(opener)名, 支持模糊搜索")
	return cmd
}

// pickOpenTarget 选择项目的打开目标（根目录 / workspaces / worktrees，1032+1030）：
// 单目标直接返回主目录；多目标交互选择，标签为「主目录」或 worktree 分支名。
func pickOpenTarget(service *project.Service, proj *project.Project) (string, error) {
	targets := service.OpenTargets(proj.Path())
	if len(targets) <= 1 {
		return proj.Path(), nil
	}

	pick, err := tui.SelectItem("选择打开目标", targets, func(t project.OpenTarget) string { return t.Label })
	if err != nil {
		if errors.Is(err, tui.ErrNotTTY) {
			return "", fmt.Errorf("该项目有 %d 个打开目标（主目录 + worktree + workspace），非 TTY 环境无法交互选择: %s", len(targets), proj.Path())
		}
		if errors.Is(err, tui.ErrUserAborted) {
			return "", fmt.Errorf("用户取消了打开目标选择: %w", err)
		}
		return "", err
	}
	return pick.Path, nil
}
