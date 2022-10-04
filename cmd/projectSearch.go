package cmd

import (
	"fmt"
	"go-cube/internal/project"
	"strings"

	"github.com/spf13/cobra"
)

var projectSearchFlags = struct {
	workspace string
	status    bool
	alfred    bool
}{}

// projectSearchCmd represents the projectSearch command
var projectSearchCmd = &cobra.Command{
	Use:   "project:search {query?* : 项目名，支持模糊匹配} {--status : 分析项目}  {--alfred : 来自 alfred 的请求}",
	Short: "搜索项目列表",
	Run: func(cmd *cobra.Command, args []string) {
		// 获取输入参数
		query := strings.Join(args, " ")
		//status := projectSearchFlags.status // todo
		alfred := projectSearchFlags.alfred

		// 项目列表
		projects := project.DefaultManager().Search(query)

		// 返回结果
		if alfred {
			var items []any
			for _, proj := range projects {
				items = append(items, map[string]string{
					"title":    proj.Name,
					"subtitle": proj.GitRepoUrl,
					"arg":      proj.Name,
				})
			}
			alfredSearchResult(items)
		} else {
			header := []string{
				fmt.Sprintf("项目(%d)", len(projects)),
				"路径",
			}
			body := make([][]string, len(projects))
			for index, proj := range projects {
				body[index] = []string{
					proj.Name,
					proj.Path,
				}
			}

			printTable(header, body)
		}

		//var maxNameLen int
		//for _, proj := range projects {
		//	if maxNameLen < len(proj.Name) {
		//		maxNameLen = len(proj.Name)
		//	}
		//}
		//
		//// 展示数据
		//for index, proj := range projects {
		//	fmt.Printf("[%3d] %s %s\n", index, strRightPad(proj.Name, maxNameLen), proj.Path)
		//}
	},
}

func init() {
	rootCmd.AddCommand(projectSearchCmd)

	// Here you will define your flags and configuration settings.
	projectSearchCmd.Flags().StringVarP(&projectSearchFlags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
	projectSearchCmd.Flags().BoolVar(&projectSearchFlags.status, "status", false, "分析项目")
	projectSearchCmd.Flags().BoolVar(&projectSearchFlags.alfred, "alfred", false, "来自 alfred 的请求")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectSearchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectSearchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func strRightPad(s string, minLen int) string {
	if len(s) >= minLen {
		return s
	} else {
		return s + strings.Repeat(" ", minLen-len(s))
	}
}
