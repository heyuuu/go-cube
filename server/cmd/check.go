package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/project"
	"cube/util/pathkit"
	"cube/util/slicekit"
	"cube/util/tui"
)

const (
	checkItemCloneRules = "clone-rules"
	checkItemGitDirty   = "git-dirty"
)

var allCheckItems = []string{checkItemCloneRules, checkItemGitDirty}

// cmd `cube check`
func newCheckCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [options...]",
		Short: "检查项目(目前 options 有: clone-rules)，不传会检查所有项目",
		Long: `对已收录的项目做批量体检，找出需要关注的项目。

目前支持的检查项：
  - clone-rules：repoUrl 能匹配 clone 规则、但实际路径
    与规则预期路径不一致的项目。
  - git-dirty：未与 remote 默认分支保持一致的项目，
    包括不在默认分支、有 ahead/behind 差异、工作区 dirty。

不传 options 时，依次执行全部检查项。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			options := args
			if len(options) == 0 {
				options = allCheckItems
			}

			for _, option := range options {
				switch option {
				case checkItemCloneRules:
					checkCloneRules(a.ProjectService())
					break
				case checkItemGitDirty:
					checkGitDirty(a.ProjectService())
				default:
					return fmt.Errorf("未支持的 option: %s", option)
				}
			}
			fmt.Print("\n\n")
			return nil
		},
	}
	return cmd
}

// 过滤出所有不符合 cloneRules 的项目
func checkCloneRules(service *project.Service) {
	projects := service.Projects()

	var headers = []string{"Name", "Path", "预期 Path", "RepoUrl"}
	var rows [][]string
	for _, p := range projects {
		info, ok := service.GitInfo(p.Path())
		repoUrl := ""
		if ok {
			repoUrl = info.RepoUrl
		}
		if repoUrl == "" {
			continue
		}

		_, localPath, ok := service.MatchCloneRule(repoUrl)

		// 有预期本地路径，但和目前实际路径不符合的情况下
		if ok && p.Path() != localPath {
			rows = append(rows, []string{
				p.Name(),
				pathkit.PrettyPath(p.Path()),
				pathkit.PrettyPath(localPath),
				repoUrl,
			})
		}
	}

	if len(rows) == 0 {
		fmt.Printf("> 没有不符合 clone rules 的项目\n")
		return
	}

	fmt.Printf("> 不符合 clone rules 的项目 %d 个:\n", len(rows))

	tui.PrintTable(headers, rows)
}

// 过滤出未与 remote 默认分支保持一致的项目
func checkGitDirty(service *project.Service) {
	projects := service.Projects()

	targets := slicekit.Filter(projects, func(p *project.Project) bool {
		info, ok := service.GitInfo(p.Path())
		if !ok || len(info.RepoUrl) == 0 {
			return false
		}

		// 分支不对
		if info.CurrentBranch != info.DefaultBranch {
			return true
		}

		// 与 remote 有差异
		if info.Ahead != 0 || info.Behind != 0 {
			return true
		}

		// 工作区 dirty
		if info.Dirty {
			return true
		}

		return false
	})

	fmt.Printf("> github dirty 的项目 %d 个:\n", len(targets))
	showProjects(service, targets)
}
