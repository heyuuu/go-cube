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

			// 返回结果
			return PrintResult(projects, func(proj *project.Project) Item {
				return Item{
					Title:    proj.Name(),
					SubTitle: proj.Path(),
					Arg:      proj.Name(),
				}
			})
		},
	}
	return cmd
}
