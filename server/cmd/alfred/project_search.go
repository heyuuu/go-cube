package alfred

import (
	"slices"
	"strings"
	"time"

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
			projects = sortProjectsByUsage(projects, latest, 10)

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

// sortProjectsByUsage 最近使用的项目置顶（按最近使用倒序，最多 limit 条），其余保持原序。
// 排序键是项目路径（usage 记录的 project 字段）。
func sortProjectsByUsage(projects []*project.Project, latest map[string]time.Time, limit int) []*project.Project {
	pinned := make([]string, 0, len(latest))
	for path := range latest {
		pinned = append(pinned, path)
	}
	slices.SortFunc(pinned, func(a, b string) int {
		return latest[b].Compare(latest[a])
	})
	if len(pinned) > limit {
		pinned = pinned[:limit]
	}

	weights := make(map[string]int, len(projects))
	for i, proj := range projects {
		weights[proj.Path()] = i + len(pinned)
	}
	for i, path := range pinned {
		weights[path] = i
	}

	slices.SortFunc(projects, func(a, b *project.Project) int {
		return weights[a.Path()] - weights[b.Path()]
	})

	return projects
}
