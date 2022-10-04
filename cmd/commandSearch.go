package cmd

import (
	"fmt"
	"go-cube/internal/command"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var commandSearchFlags = struct {
	project string
	alfred  bool
}{}

// commandSearchCmd represents the commandSearch command
var commandSearchCmd = &cobra.Command{
	Use:   "command:search {query? : 命令名，支持模糊匹配} {--project= : 项目名} {--alfred : 来自 alfred 的请求}",
	Short: "搜索可用命令列表",
	Run: func(cmd *cobra.Command, args []string) {
		query := args
		projectName := commandSearchFlags.project
		alfred := commandSearchFlags.alfred

		// 获取匹配的命令列表
		var commands []command.Command
		if len(query) == 0 {
			commands = command.DefaultManager().Commands()
			sort.Slice(commands, func(i, j int) bool {
				return commands[i].Name < commands[j].Name
			})
		} else {
			commands = command.DefaultManager().Search(strings.Join(query, " "))
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
}

func init() {
	rootCmd.AddCommand(commandSearchCmd)

	// Here you will define your flags and configuration settings.
	rootCmd.PersistentFlags().StringVar(&commandSearchFlags.project, "project", "", "项目名")
	rootCmd.PersistentFlags().BoolVar(&commandSearchFlags.alfred, "alfred", false, "来自 alfred 的请求")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// commandSearchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// commandSearchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
