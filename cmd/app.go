package cmd

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/project"
	"github.com/heyuuu/go-cube/internal/slicekit"
	"strings"

	"github.com/spf13/cobra"
)

var appCmd = initCmd(cmdOpts[any]{
	Use: "app",
})

// cmd `app list`
var appListCmd = initCmd(cmdOpts[any]{
	Root:  appCmd,
	Use:   "list",
	Short: "列出可用命令列表",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		apps := app.DefaultManager().Apps()
		showApps(apps)
	},
})

// cmd `app search`
type appSearchFlags struct {
	project string
}

var appSearchCmd = initCmd(cmdOpts[appSearchFlags]{
	Root:  appCmd,
	Use:   "search {query? : 命令名，支持模糊匹配} {--project= : 项目名} {--alfred : 来自 alfred 的请求}",
	Short: "搜索可用命令列表",
	Init: func(cmd *cobra.Command, flags *appSearchFlags) {
		cmd.PersistentFlags().StringVar(&flags.project, "project", "", "项目名")
	},
	Run: func(cmd *cobra.Command, flags *appSearchFlags, args []string) {
		query := args
		projectName := flags.project

		// 获取匹配的命令列表
		var apps []app.App
		if len(query) == 0 {
			apps = app.DefaultManager().Apps()
		} else {
			apps = app.DefaultManager().Search(strings.Join(query, " "))
		}

		// 若指定项目，且对应空间有指定命令优先级，则按优先级排序
		var preferApps []string
		if len(projectName) > 0 {
			pm := project.DefaultManager()
			ws := pm.FindWorkspaceByProjectName(projectName)
			preferApps = ws.PreferApps()
		}
		apps = sortApps(apps, preferApps)

		// 返回结果
		showApps(apps)
	},
})

func showApps(apps []app.App) {
	if isAlfred {
		alfredSearchResultFunc(apps, app.App.Name, app.App.Bin, app.App.Name)
	} else {
		header := []string{
			fmt.Sprintf("项目(%d)", len(apps)),
			"路径",
		}
		printTableFunc(apps, header, app.App.Name, app.App.Bin)
	}
}

func sortApps(apps []app.App, preferAppNames []string) []app.App {
	if len(apps) <= 1 || len(preferAppNames) == 0 {
		return apps
	}

	preferAppNameMap := make(map[string]int, len(preferAppNames))
	for i, appName := range preferAppNames {
		preferAppNameMap[appName] = i
	}

	return slicekit.SortByWithIndex(apps, func(i int, app app.App) int {
		if idx, ok := preferAppNameMap[app.Name()]; ok {
			return idx
		} else {
			return i + len(preferAppNames)
		}
	})
}
