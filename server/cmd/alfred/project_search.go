package alfred

import (
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
)

// cmd `alfred project-search`
func newProjectSearchCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project-search {query?* : 项目名，支持模糊匹配}",
		Short: "搜索项目列表",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取输入参数
			query := strings.Join(args, " ")

			// 项目列表
			projects := a.ProjectService().SearchByName(query)

			// 最近使用的项目置顶
			latest := a.UsageService().LatestByProject()
			projects = project.SortByRecentUsage(projects, latest, 10)

			// 平铺直达（1032）：每个项目展开为「根目录 + worktrees + workspaces」多个条目，
			// Arg 直接传目标目录路径，保留一步打开体验（无需先选项目再选目标）
			items := make([]Item, 0, len(projects))
			for _, proj := range projects {
				items = append(items, Item{
					Title:    proj.Name(),
					SubTitle: proj.Path(),
					Arg:      proj.Path(),
				})
				for _, target := range a.ProjectService().OpenTargets(proj.Path())[1:] { // 跳过根目录（上面已输出）
					items = append(items, Item{
						Title:    proj.Name() + " (" + target.Label + ")",
						SubTitle: target.Path,
						Arg:      target.Path,
					})
				}
			}
			return PrintItems(items)
		},
	}
	return cmd
}
