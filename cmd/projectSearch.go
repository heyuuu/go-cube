package cmd

import (
	"fmt"
	"go-cube/internal/project"
	"sort"
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
	Use:   "project:search [query]",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		projects := project.DefaultManager().Search(query)

		// 排序
		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})

		var maxNameLen int
		for _, proj := range projects {
			if maxNameLen < len(proj.Name) {
				maxNameLen = len(proj.Name)
			}
		}

		// 展示数据
		for index, proj := range projects {
			fmt.Printf("[%3d] %s %s\n", index, strRightPad(proj.Name, maxNameLen), proj.Path)
		}
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
