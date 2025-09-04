package alfred

import (
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
	"strings"
)

// cmd `alfred search`
var projectSearchCmd = &easycobra.Command{
	Use:   "project-search {query?* : 项目名，支持模糊匹配}",
	Short: "搜索项目列表",
	Run: func(cmd *cobra.Command, args []string) {
		// 获取输入参数
		query := strings.Join(args, " ")

		// 项目列表
		service := app.Default().ProjectService()
		projects := service.Search(query)

		// 返回结果
		PrintResultFunc(projects, func(proj *entities.Project) Item {
			return Item{
				Title:    proj.Name(),
				SubTitle: proj.RepoUrl(),
				Arg:      proj.Name(),
			}
		})
	},
}
