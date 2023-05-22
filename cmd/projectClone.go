package cmd

import (
	"github.com/spf13/cobra"
)

var projectCloneFlags = struct {
	depth  int
	branch string
}{}

// projectCloneCmd represents the projectClone command
var projectCloneCmd = &cobra.Command{
	Use:   "project:clone {repoUrl} {--depth= : 克隆深度，默认为不限制} {--b|branch=}",
	Short: "使用 RepoUrl 初始化项目",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		//repoUrlStr := args[0]
		//depth := projectCloneFlags.depth
		//branch := projectCloneFlags.branch
		//if len(branch) != 0 && depth < 0 {
		//	depth = 1 // 指定分支情况下，默认深度为1
		//}
		//
		//// 匹配hub
		//repoUrl =

	},
}

func init() {
	rootCmd.AddCommand(projectCloneCmd)

	// Here you will define your flags and configuration settings.
	rootCmd.Flags().IntVar(&projectCloneFlags.depth, "depth", -1, "克隆深度，默认为不限制")
	rootCmd.Flags().StringVarP(&projectCloneFlags.branch, "branch", "b", "", "分支名，默认为master")

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectCloneCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectCloneCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
