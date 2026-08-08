package project

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
	"cube/util/pathkit"
	"cube/util/tui"
)

// cmd `project list`
func newListCmd(a *app.App) *cobra.Command {
	var verbose int
	var group string
	cmd := &cobra.Command{
		Use:   "list [query]",
		Short: "项目列表(支持模糊搜索)",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 获取输入参数
			var query string
			if len(args) > 0 {
				query = args[0]
			}

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
			showProjects(projects, verbose)

			return nil
		},
	}
	cmd.Flags().CountVarP(&verbose, "verbose", "v", "verbose level (-v, -vv, -vvv)")
	cmd.Flags().StringVarP(&group, "group", "g", "", "限定分组 group")
	return cmd
}

func showProjects(projects []*project.Project, verbose int) {
	var headers []string
	rows := make([][]string, len(projects))

	// verbose: 0
	headers = append(headers, fmt.Sprintf("项目(%d)", len(projects)), "Path", "RepoUrl")
	for i, p := range projects {
		rows[i] = append(rows[i], p.Name(), pathkit.PrettyPath(p.Path()), p.RepoUrl())
	}

	// verbose: 1
	if verbose >= 1 {
		headers = append(headers, "当前分支", "默认分支", "默认分支差异", "当前工作区是否干净")
		for i, p := range projects {
			info := p.GitInfo()

			var currBranch, defaultBranch, branchDiff, statusText string
			if info != nil {
				currBranch = info.CurrentBranch
				defaultBranch = info.DefaultBranch
				if info.Ahead != 0 {
					branchDiff += "+" + strconv.Itoa(info.Ahead)
				}
				if info.Behind != 0 {
					branchDiff += "-" + strconv.Itoa(info.Behind)
				}
				if info.Dirty {
					statusText = "dirty"
				}
			}

			rows[i] = append(rows[i], currBranch, defaultBranch, branchDiff, statusText)
		}
	}

	// 输出表格
	tui.PrintTable(headers, rows)
}
