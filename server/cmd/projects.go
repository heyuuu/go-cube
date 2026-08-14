package cmd

import (
	"fmt"
	"slices"

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
		Short: "项目列表(支持模糊搜索)",
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
			projects := searchProjects(a.ProjectService(), query, false)

			// 按 group 过滤
			if len(group) > 0 {
				projects = slices.DeleteFunc(projects, func(p *project.Project) bool {
					return p.Group() != group
				})
			}

			// 展示项目列表
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
