package cmd

import (
	"fmt"
	"go-cube/internal/app"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var appCmd = initCmd(cmdOpts[any]{
	Use: "app",
})

// cmd `app list`
var appListCmd = initCmd(cmdOpts[any]{
	Root: appCmd,
	Use:  "list",
	Run: func(cmd *cobra.Command, flags *any, args []string) {
		fmt.Println("app list called")
	},
})

// cmd `app search`
type appSearchFlags struct {
	project string
	alfred  bool
}

var appSearchCmd = initCmd(cmdOpts[appSearchFlags]{
	Use:   "search {query? : 命令名，支持模糊匹配} {--project= : 项目名} {--alfred : 来自 alfred 的请求}",
	Short: "搜索可用命令列表",
	Init: func(cmd *cobra.Command, flags *appSearchFlags) {
		cmd.PersistentFlags().StringVar(&flags.project, "project", "", "项目名")
		cmd.PersistentFlags().BoolVar(&flags.alfred, "alfred", false, "来自 alfred 的请求")
	},
	Run: func(cmd *cobra.Command, flags *appSearchFlags, args []string) {
		query := args
		projectName := flags.project
		alfred := flags.alfred

		// 获取匹配的命令列表
		var commands []app.App
		if len(query) == 0 {
			commands = app.DefaultManager().Apps()
			sort.Slice(commands, func(i, j int) bool {
				return commands[i].Name < commands[j].Name
			})
		} else {
			commands = app.DefaultManager().Search(strings.Join(query, " "))
		}

		// 若指定项目，且对应空间有指定命令优先级，则按优先级排序
		if len(projectName) > 0 {
			// todo
		}

		// 返回结果
		if alfred {
			var items []any
			for _, cmd := range commands {
				items = append(items, map[string]string{
					"title":    cmd.Name,
					"subtitle": cmd.Bin,
					"arg":      cmd.Name,
				})
			}
			alfredSearchResult(items)
		} else {
			header := []string{
				fmt.Sprintf("项目(%d)", len(commands)),
				"路径",
			}
			body := make([][]string, len(commands))
			for index, cmd := range commands {
				body[index] = []string{
					cmd.Name,
					cmd.Bin,
				}
			}
			printTable(header, body)
		}
	},
})
