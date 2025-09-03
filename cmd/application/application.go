package application

import (
	"fmt"
	"github.com/heyuuu/go-cube/cmd/alfred"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/heyuuu/go-cube/internal/util/slicekit"
	"github.com/spf13/cobra"
	"strings"
)

var AppCmd = &easycobra.Command[any]{
	Use:     "application",
	Aliases: []string{"app"},
}

func init() {
	easycobra.AddCommand(AppCmd, appListCmd)
	easycobra.AddCommand(AppCmd, appSearchCmd)
}

// cmd `app list`
var appListCmd = &easycobra.Command[any]{
	Use:   "list",
	Short: "列出可用命令列表",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		service := app.Default().ApplicationService()
		apps := service.Apps()
		showApps(apps)
	},
}

// cmd `app search`
type appSearchFlags struct {
	project string
}

var appSearchCmd = &easycobra.Command[appSearchFlags]{
	Use:   "search {query? : 命令名，支持模糊匹配} {--project= : 项目名} {--alfred : 来自 alfred 的请求}",
	Short: "搜索可用命令列表",
	Init: func(cmd *cobra.Command, flags *appSearchFlags) {
		cmd.PersistentFlags().StringVar(&flags.project, "project", "", "项目名")
	},
	Run: func(cmd *cobra.Command, flags *appSearchFlags, args []string) {
		query := args
		projectName := flags.project

		// 获取匹配的命令列表
		service := app.Default().ApplicationService()
		apps := service.Search(strings.Join(query, " "))

		// 若指定项目，且对应空间有指定命令优先级，则按优先级排序
		var preferApps []string
		if len(projectName) > 0 {
			service := app.Default().ProjectService()
			ws := service.FindWorkspaceByProjectName(projectName)
			preferApps = ws.PreferApps()
		}
		apps = sortApps(apps, preferApps)

		// 返回结果
		showApps(apps)
	},
}

func showApps(apps []*entities.Application) {
	if alfred.IsAlfred {
		alfred.PrintResultFunc(apps, func(item *entities.Application) alfred.Item {
			return alfred.Item{
				Title:    item.Name(),
				SubTitle: item.Bin(),
				Arg:      item.Name(),
			}
		})
	} else {
		header := []string{
			fmt.Sprintf("项目(%d)", len(apps)),
			"路径",
		}
		console.PrintTableFunc(apps, header, func(app *entities.Application) []string {
			return []string{
				app.Name(),
				app.Bin(),
			}
		})
	}
}

func sortApps(apps []*entities.Application, preferAppNames []string) []*entities.Application {
	if len(apps) <= 1 || len(preferAppNames) == 0 {
		return apps
	}

	preferAppNameMap := make(map[string]int, len(preferAppNames))
	for i, appName := range preferAppNames {
		preferAppNameMap[appName] = i
	}

	return slicekit.SortByWithIndex(apps, func(i int, app *entities.Application) int {
		if idx, ok := preferAppNameMap[app.Name()]; ok {
			return idx
		} else {
			return i + len(preferAppNames)
		}
	})
}
