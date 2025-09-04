package application

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/spf13/cobra"
	"strings"
)

var AppCmd = &easycobra.Command{
	Use:     "application",
	Aliases: []string{"app"},
}

func init() {
	AppCmd.AddCommand(appListCmd)
	AppCmd.AddCommand(appSearchCmd)
}

// cmd `app list`
var appListCmd = &easycobra.Command{
	Use:   "list",
	Short: "列出可用命令列表",
	Run: func(cmd *cobra.Command, args []string) {
		service := app.Default().ApplicationService()
		apps := service.Apps()
		showApps(apps)
	},
}

// cmd `app search`
var appSearchCmd = &easycobra.Command{
	Use:   "search {query? : 命令名，支持模糊匹配}",
	Short: "搜索可用命令列表",
	Run: func(cmd *cobra.Command, args []string) {
		query := args

		// 获取匹配的命令列表
		service := app.Default().ApplicationService()
		apps := service.Search(strings.Join(query, " "))

		// 返回结果
		showApps(apps)
	},
}

func showApps(apps []*entities.Application) {
	console.PrintTableFunc(apps, []string{
		fmt.Sprintf("项目(%d)", len(apps)),
		"路径",
	}, func(app *entities.Application) []string {
		return []string{
			app.Name(),
			app.Bin(),
		}
	})
}
