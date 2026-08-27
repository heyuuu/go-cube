package cmd

import (
	"fmt"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
	"cube/util/pathkit"
	"cube/util/slicekit"
	"cube/util/tui"
)

func newProjectsCmd(a *app.App) *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "projects [query] [-g|--group=组名]",
		Short: "项目列表(支持项目名或项目路径模糊搜索)",
		Long: `显示项目列表，支持按项目名称或项目路径进行搜索。

query 支持两种搜索模式：
  - 项目名称搜索：按关键词模糊匹配项目名称（默认）。
  - 项目路径搜索：当 query 以 '.'、'~' 或 '/' 开头时触发，
    搜索给定路径及其所有子目录中的项目。

不传入 query 时，显示所有项目。`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取输入参数
			query := getArg(args, 0)

			// 项目列表
			projects, err := searchProjects(a.ProjectService(), query, false)
			if err != nil {
				return err
			}

			// 按 group 过滤
			if len(group) > 0 {
				projects = slices.DeleteFunc(projects, func(p *project.Project) bool {
					return p.Group() != group
				})
			}

			// 展示项目列表（最近使用的项目置顶）
			latest := a.UsageService().LatestByProject()
			projects = sortProjectsByUsage(projects, latest, 10)
			showProjects(a.ProjectService(), projects)

			return nil
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "限定分组 group")
	return cmd
}

// 输出表格
func showProjects(service *project.Service, projects []*project.Project) {
	tui.PrintTable(
		[]string{
			fmt.Sprintf("项目(%d)", len(projects)),
			"Path",
			"RepoUrl",
		},
		slicekit.Map(projects, func(p *project.Project) []string {
			repoUrl := ""
			if info, ok := service.GitInfo(p.Path()); ok {
				repoUrl = info.RepoUrl
			}
			return []string{
				p.Name(),
				pathkit.PrettyPath(p.Path()),
				repoUrl,
			}
		}),
	)
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
