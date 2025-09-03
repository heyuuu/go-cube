package alfred

import (
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
	"strings"
)

// cmd `alfred search`
type projectSearchFlags struct{}

var projectSearchCmd = &easycobra.Command[projectSearchFlags]{
	Use:   "project-search {query?* : 项目名，支持模糊匹配}",
	Short: "搜索项目列表",
	Run: func(cmd *cobra.Command, _ *projectSearchFlags, args []string) {
		// 获取输入参数
		query := strings.Join(args, " ")

		// 项目列表
		service := app.Default().ProjectService()
		projects := service.SearchInWorkspace(query, "")

		// 返回结果
		PrintResultFunc(projects, func(item *entities.Project) Item {
			return Item{
				Title:    item.Name(),
				SubTitle: item.RepoUrl(),
				Arg:      item.Name(),
			}
		})
	},
}
