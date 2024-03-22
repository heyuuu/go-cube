package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"go-cube/internal/project"
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
		cmd.Flags().BoolVar(&flags.alfred, "alfred", false, "来自 alfred 的请求")
	},
	Run: func(cmd *cobra.Command, flags *projectSearchFlags, args []string) {

		// 获取输入参数
		query := strings.Join(args, " ")
		//status := flags.status // todo
		alfred := flags.alfred

		// 项目列表
		projects := project.DefaultManager().Search(query)

		// 返回结果
		if alfred {
			alfredSearchResultFunc(projects, (*project.Project).Name, (*project.Project).RepoUrl, (*project.Project).Name)
		} else {
			header := []string{
				fmt.Sprintf("项目(%d)", len(projects)),
				"路径",
			}
			printTableFunc(projects, header, (*project.Project).Name, (*project.Project).RepoUrl)
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
})

// cmd `project info`
type projectInfoFlags struct {
	workspace string
}

var projectInfoCmd = initCmd(cmdOpts[projectInfoFlags]{
	Root:  projectCmd,
	Use:   "info [project]",
	Short: "",
	Args:  cobra.ExactArgs(1),
	Init: func(cmd *cobra.Command, flags *projectInfoFlags) {
		cmd.Flags().StringVarP(&flags.workspace, "workspace", "w", "", "指定工作区，默认针对所有工作区")
	},
	Run: func(cmd *cobra.Command, flags *projectInfoFlags, args []string) {
		query := args[0]
		projects := project.DefaultManager().Search(query)

		// 没有匹配时结束命令
		if len(projects) == 0 {
			fmt.Println("没有匹配的项目")
			return
		}

		for index, proj := range projects {
			fmt.Printf("[%3d] %s %s\n", index, strRightPad(proj.Name(), 20), proj.Path())
		}
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
