package alfred

import (
	"slices"
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
			service := a.ProjectService()
			projects := service.SearchByName(query)

			// 最近打开日志
			historyService := a.HistoryService()
			history := historyService.LeastSelectedProjects(10, true)
			sortProjectsWithHistory(projects, history)

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

// 优先将 history 排在前面，保持其他顺序不变
func sortProjectsWithHistory(projects []*project.Project, history []string) []*project.Project {
	weights := make(map[string]int, len(history))
	for i, proj := range projects {
		weights[proj.Name()] = i + len(history)
	}
	for i, proj := range history {
		weights[proj] = i
	}

	slices.SortFunc(projects, func(a, b *project.Project) int {
		return weights[a.Name()] - weights[b.Name()]
	})

	return projects
}
