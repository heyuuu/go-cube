package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/util/git"
)

// cmd `project clone`
var cloneCmd = &easycobra.Command{
	Use:   "clone {repoUrl} {--depth= : 克隆深度，默认为不限制} {--b|branch=}",
	Short: "使用 RepoUrl 初始化项目",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var depth int
		var branch string
		cmd.Flags().IntVar(&depth, "depth", -1, "克隆深度，默认为不限制")
		cmd.Flags().StringVarP(&branch, "branch", "b", "", "分支名，默认为master")

		// run
		return func(args []string) error {
			rawRepoUrl := args[0]
			if branch != "" && depth == 0 {
				depth = 1 // // 指定分支情况下，默认深度为1
			}

			// 预检查 rawRepoUrl 是否为合法 git repoUrl
			_, err := git.ParseRepoUrl(rawRepoUrl)
			if err != nil {
				return fmt.Errorf("repoUrl 不是合法地址: url=%s", rawRepoUrl)
			}

			// 匹配 CloneRule，获取对应本地路径
			service := app.Default().ProjectService()
			_, localPath, ok := service.MatchCloneRule(rawRepoUrl)
			if !ok {
				return fmt.Errorf("repoUrl 没有对应 clone 规则: url=%s", rawRepoUrl)
			}

			// 执行命令
			err = git.Clone(localPath, rawRepoUrl, depth, branch)
			if err != nil {
				return fmt.Errorf("执行 clone 命令失败: %s", err)
			}

			return nil
		}
	},
}
