package cmd

import (
	"slices"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
)

func newProjectsCmd(a *app.App) *cobra.Command {
	var group string
	cmd := &cobra.Command{
		Use:   "projects [query] [-g|--group=组名]",
		Short: "项目列表(支持模糊搜索)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取输入参数
			query := getArg(args, 0)

			// 项目列表
			service := a.ProjectService()
			projects := service.Search(query)

			// 按 group 过滤
			if len(group) > 0 {
				projects = slices.DeleteFunc(projects, func(p *project.Project) bool {
					return p.Group() != group
				})
			}

			// 展示项目列表
			showProjects(projects)

			return nil
		},
	}
	cmd.Flags().StringVarP(&group, "group", "g", "", "限定分组 group")
	return cmd
}
