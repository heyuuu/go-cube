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
	Use:   "info [project]",
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
	projectCmd.AddCommand(projectInfoCmd)

	// Here you will define your flags and configuration settings.
	projectInfoCmd.Flags().StringVarP(&projectInfoFlags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
}
