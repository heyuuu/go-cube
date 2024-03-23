package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-cube/internal/app"
	"go-cube/internal/git"
	"go-cube/internal/project"
	"go-cube/internal/slicekit"
	"log"
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
		projects := project.DefaultManager().SearchInWorkspace(query, flags.workspace)

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
							p.Path(),
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

		// 匹配项目
		proj := selectProject(query, flags.workspace)
		if proj == nil {
			return
		}

		fmt.Printf("project: %s\n", proj.Name())
		fmt.Printf("path   : %s\n", proj.Path())
		fmt.Printf("git-url: %s\n", proj.RepoUrl())
	},
})

// cmd `project open`
type projectOpenFlags struct {
	workspace string
	app       string
}

var projectOpenCmd = initCmd(cmdOpts[projectOpenFlags]{
	Root:  projectCmd,
	Use:   "open {project : 项目名} {--app= : 打开项目的App} {--w|workspace= : 指定工作区，仅交互模式有意义}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	Init: func(cmd *cobra.Command, flags *projectOpenFlags) {
		cmd.Flags().StringVarP(&flags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
		cmd.Flags().StringVar(&flags.app, "app", "", "打开项目的App")
	},
	Run: func(cmd *cobra.Command, flags *projectOpenFlags, args []string) {
		query := args[0]

		// 获取打开项目的app
		openApp, ok := app.DefaultManager().FindApp(flags.app)
		if !ok {
			log.Fatal("未找到指定app: " + flags.app)
			return
		}

		// 匹配项目
		proj := selectProject(query, flags.workspace)
		if proj == nil {
			return
		}

		// 打开项目
		err := passthruRun(openApp.Bin(), proj.Path())
		if err != nil {
			log.Fatalf("打开失败: " + err.Error())
		}
	},
})

func selectProject(query string, workspace string) *project.Project {
	projects := project.DefaultManager().SearchInWorkspace(query, workspace)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, ok := choiceItem("选择项目", projects, (*project.Project).Name)
		if !ok {
			fmt.Println("选择项目失败")
			return nil
		}
		return proj
	}
}

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
