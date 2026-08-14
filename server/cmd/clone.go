package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/util/git"
)

// cmd `cube clone`
func newCloneCmd(a *app.App) *cobra.Command {
	var depth int
	var branch string
	cmd := &cobra.Command{
		Use:   "clone [repoUrl [--depth=克隆深度，默认为不限制] [--b|branch=分支名]",
		Short: "使用 RepoUrl 初始化项目",
		Long: `按 clone 规则克隆仓库，自动落地到规则推导出的本地路径。

repoUrl 必须是合法的 git 仓库地址，且能匹配到一条 clone 规则；
任一不满足都会直接报错退出，不会在当前目录下随意克隆。

指定 --branch 时若未显式给 --depth，默认克隆深度为 1。`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawRepoUrl := args[0]
			if branch != "" && depth == 0 {
				depth = 1 // 仅显式 --depth=0 且指定分支时触发；flag 默认 -1（不限制）
			}

			// 预检查 rawRepoUrl 是否为合法 git repoUrl
			_, err := git.ParseRepoUrl(rawRepoUrl)
			if err != nil {
				return fmt.Errorf("repoUrl 不是合法地址: url=%s", rawRepoUrl)
			}

			// 匹配 CloneRule，获取对应本地路径
			service := a.ProjectService()
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
		},
	}
	cmd.Flags().IntVar(&depth, "depth", -1, "克隆深度，默认为不限制")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "分支名，默认为master")
	return cmd
}
