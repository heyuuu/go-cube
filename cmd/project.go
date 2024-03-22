package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-cube/internal/git"
	"go-cube/internal/project"
	"go-cube/internal/slicekit"
	"slices"
	"strconv"
	"strings"
)

// cmd group `project *`
var projectCmd = initCmd(cmdOpts[any]{
	Use:     "project",
	Aliases: []string{"proj", "p"},
})

// cmd `project search`
type projectSearchFlags struct {
	workspace string
	status    bool
	alfred    bool
}

var projectSearchCmd = initCmd(cmdOpts[projectSearchFlags]{
	Root:  projectCmd,
	Use:   "search {query?* : 项目名，支持模糊匹配} {--status : 分析项目}  {--alfred : 来自 alfred 的请求}",
	Short: "搜索项目列表",
	Init: func(cmd *cobra.Command, flags *projectSearchFlags) {
		cmd.Flags().StringVarP(&flags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
		cmd.Flags().BoolVar(&flags.status, "status", false, "分析项目")
	},
	Run: func(cmd *cobra.Command, flags *projectSearchFlags, args []string) {
		// 获取输入参数
		query := strings.Join(args, " ")
		showStatus := flags.status

		// 项目列表
		projects := project.DefaultManager().Search(query)

		// 返回结果
		if isAlfred {
			alfredSearchResultFunc(projects, (*project.Project).Name, (*project.Project).RepoUrl, (*project.Project).Name)
		} else {
			if !showStatus {
				printTable(
					[]string{
						fmt.Sprintf("项目(%d)", len(projects)),
						"路径",
					},
					slicekit.Map(projects, func(p *project.Project) []string {
						return []string{
							p.Name(),
							p.RepoUrl(),
						}
					}),
				)
			} else {
				printTable(
					[]string{
						fmt.Sprintf("项目(%d)", len(projects)),
						"路径",
						"当前分支",
						"Master差异",
						"当前工作区是否干净",
					},
					slicekit.Map(projects, func(p *project.Project) []string {
						branches, currBranch, _ := git.Branches(p.Path(), true, true)

						var branchDiff string
						if slices.Contains(branches, "master") && slices.Contains(branches, "origin/master") {
							forward, _ := git.LogBetweenCount(p.Path(), "master", "origin/master")
							backward, _ := git.LogBetweenCount(p.Path(), "origin/master", "master")
							if forward != 0 {
								branchDiff += "+" + strconv.Itoa(forward)
							}
							if backward != 0 {
								branchDiff += "-" + strconv.Itoa(backward)
							}
						}

						var statusText string
						if isDirty, _ := git.IsDirty(p.Path()); isDirty {
							statusText = "dirty"
						}

						return []string{
							p.Name(),
							p.Path(),
							currBranch,
							branchDiff,
							statusText,
						}
					}),
				)
			}
		}
	},
})

// cmd `project info`
type projectInfoFlags struct {
	workspace string
}

var projectInfoCmd = initCmd(cmdOpts[projectInfoFlags]{
	Root:  projectCmd,
	Use:   "info {project : 项目名，支持模糊匹配} {--w|workspace= : 指定工作区，默认针对所有工作区}",
	Short: "打开项目",
	Args:  cobra.ExactArgs(1),
	Init: func(cmd *cobra.Command, flags *projectInfoFlags) {
		cmd.Flags().StringVarP(&flags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
	},
	Run: func(cmd *cobra.Command, flags *projectInfoFlags, args []string) {
		query := args[0]

		var projects []*project.Project
		if flags.workspace == "" {
			projects = project.DefaultManager().Search(query)
		} else {
			projects = project.DefaultManager().SearchInWorkspace(query, flags.workspace)
		}

		// 没有匹配时结束命令
		var proj *project.Project
		switch len(projects) {
		case 0:
			fmt.Println("没有匹配的项目")
			return
		case 1:
			proj = projects[0]
		default:
			var ok bool
			proj, ok = choiceItem("选择项目", projects, (*project.Project).Name)
			if !ok {
				fmt.Println("选择项目失败")
				return
			}
		}

		fmt.Printf("project: %s\n", proj.Name())
		fmt.Printf("path   : %s\n", proj.Path())
		fmt.Printf("git-url: %s\n", proj.RepoUrl())
	},
})

// cmd `project open`
type projectOpenFlags struct {
}

var projectOpenCmd = initCmd(cmdOpts[projectOpenFlags]{
	Root: projectCmd,
	Use:  "open",
	Init: func(cmd *cobra.Command, flags *projectOpenFlags) {},
	Run: func(cmd *cobra.Command, flags *projectOpenFlags, args []string) {
		fmt.Println("projectOpen called")
	},
})

// cmd `project clone`
type projectCloneFlags struct {
	depth  int
	branch string
}

var projectCloneCmd = initCmd(cmdOpts[projectCloneFlags]{
	Root:  projectCmd,
	Use:   "clone {repoUrl} {--depth= : 克隆深度，默认为不限制} {--b|branch=}",
	Short: "使用 RepoUrl 初始化项目",
	Args:  cobra.ExactArgs(1),
	Init: func(cmd *cobra.Command, flags *projectCloneFlags) {
		cmd.Flags().IntVar(&flags.depth, "depth", -1, "克隆深度，默认为不限制")
		cmd.Flags().StringVarP(&flags.branch, "branch", "b", "", "分支名，默认为master")
	},
	Run: func(cmd *cobra.Command, flags *projectCloneFlags, args []string) {
		//repoUrlStr := args[0]
		//depth := flags.depth
		//branch := flags.branch
		//if len(branch) != 0 && depth < 0 {
		//	depth = 1 // 指定分支情况下，默认深度为1
		//}
		//
		//// 匹配hub
		//repoUrl =

	},
})
