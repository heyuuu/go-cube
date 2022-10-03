package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-cube/internal/project"
)

var projectInfoFlags = struct {
	workspace string
}{}

// projectInfoCmd represents the projectInfo command
var projectInfoCmd = &cobra.Command{
	Use:   "project:info [project]",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := args[0]
		projects := project.DefaultManager().Search(query)

		// 没有匹配时结束命令
		if len(projects) == 0 {
			fmt.Println("没有匹配的项目")
			return
		}

		for index, proj := range projects {
			fmt.Printf("[%3d] %s %s\n", index, strRightPad(proj.Name, 20), proj.Path)
		}
	},
}

func init() {
	rootCmd.AddCommand(projectInfoCmd)

	// Here you will define your flags and configuration settings.
	projectInfoCmd.Flags().StringVarP(&projectSearchFlags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectInfoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectInfoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
