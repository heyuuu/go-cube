package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `project info`
func newInfoCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [query]",
		Short: "打开项目(支持模糊搜索)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			// 匹配项目
			proj := selectProject(a.ProjectService(), query)
			if proj == nil {
				return nil
			}

			fmt.Printf("project: %s\n", proj.Name())
			fmt.Printf("path   : %s\n", proj.Path())
			// repoUrl 读 git 缓存（Project.RepoUrl 字段已废弃）
			var repoUrl string
			if info, ok := a.ProjectService().GitInfo(proj.Path()); ok {
				repoUrl = info.RepoUrl
			}
			fmt.Printf("git-url: %s\n", repoUrl)

			return nil
		},
	}
	return cmd
}
