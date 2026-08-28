package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project/cubefile"
	"cube/project/workspace"
	"cube/util/tui"
)

// cmd `cube workspace init`（提案 1030）：探测标准 monorepo 声明 → 挑选成员 →
// 固化为 .cube/cube.json 的显式 workspaces（init 的语义就是固化，不提供只写 scanRule 的选项）。
func newWorkspaceCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "monorepo workspace 声明管理",
	}
	cmd.AddCommand(newWorkspaceInitCmd(a))
	return cmd
}

func newWorkspaceInitCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [query]",
		Short: "探测 monorepo 声明并挑选成员，写入 .cube/cube.json",
		Long: `探测项目根下的标准 monorepo 声明文件（pnpm-workspace.yaml / package.json workspaces，
规则同打开目标探测：优先 cube.json 的 workspaceScanRule，无则默认 pnpm,npm），
交互挑选成员后固化为显式 workspaces 声明。声明进 git 跟仓库走。

query 支持项目名模糊搜索，规则同 open 命令。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := pickProject(a.ProjectService(), getArg(args, 0))
			if err != nil {
				return err
			}
			root := proj.Path()

			// 已有显式声明时确认覆盖（探测产的是候选全集，覆盖会丢失人工裁剪）
			if cf, ok := cubefile.Load(root); ok && cf.WorkspacesSet {
				overwrite, err := tui.Confirm("已存在显式 workspaces 声明，重新探测并覆盖？")
				if err != nil {
					return err
				}
				if !overwrite {
					return errors.New("用户取消了覆盖")
				}
			}

			// 探测用「与采集一致的规则」，保证 init 看到的候选就是无声明时的正选
			members := workspace.Detect(root, workspace.EffectiveScanRule(root))
			if len(members) == 0 {
				return fmt.Errorf("未探测到标准 monorepo 声明（pnpm-workspace.yaml / package.json workspaces），可手写 %s/.cube/cube.json", root)
			}

			picked, err := tui.MultiSelectItemWithDefaults(
				"挑选 workspace 成员（固化为显式声明）",
				members,
				func(w workspace.Workspace) string { return w.Name + "  (" + w.Path + ")" },
				members, // 默认全选：探测命中即认为都是成员，裁剪是减法
			)
			if err != nil {
				return err
			}
			if len(picked) == 0 {
				return errors.New("未选择任何成员，未写入")
			}

			if err := workspace.Save(root, picked); err != nil {
				return err
			}
			// 定向重采集：写盘即刻反映到打开目标，不等 TTL
			if err := a.ProjectService().RefreshGitInfo(root); err != nil {
				fmt.Printf("cube.json 已写入，但刷新快照失败（等下次采集自愈）: %v\n", err)
			}
			fmt.Printf("已写入 %s/.cube/cube.json（%d 个 workspace），记得提交进 git\n", root, len(picked))
			return nil
		},
	}
	return cmd
}
