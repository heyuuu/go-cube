package project

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/app"
	"github.com/heyuuu/go-cube/internal/entities"
	"github.com/heyuuu/go-cube/internal/util/console"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"github.com/heyuuu/go-cube/internal/util/git"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// cmd group `project *`
var ProjectCmd = &easycobra.Command{
	Use:     "project",
	Aliases: []string{"proj", "p"},
}

func init() {
	ProjectCmd.AddCommand(projectSearchCmd)
	ProjectCmd.AddCommand(projectInfoCmd)
	ProjectCmd.AddCommand(projectOpenCmd)
	ProjectCmd.AddCommand(projectCloneCmd)
}

// cmd `project search`
var projectSearchCmd = &easycobra.Command{
	Use:   "search {query?* : 项目名，支持模糊匹配} {--status : 分析项目}",
	Short: "搜索项目列表",
	InitRun: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) {
		// init flags
		var workspaceName string
		var showStatus bool
		cmd.Flags().StringVarP(&workspaceName, "workspace", "w", "", "指定工作区，默认针对所有工作区")
		cmd.Flags().BoolVar(&showStatus, "status", false, "分析项目")

		// run
		return func(cmd *cobra.Command, args []string) {
			// 获取输入参数
			query := strings.Join(args, " ")

			// 项目列表
			service := app.Default().ProjectService()
			projects := service.SearchInWorkspace(query, workspaceName)

			// 返回结果
			if !showStatus {
				console.PrintTableFunc(projects, []string{
					fmt.Sprintf("项目(%d)", len(projects)),
					"路径",
				}, func(p *entities.Project) []string {
					return []string{
						p.Name(),
						p.Path(),
					}
				})
			} else {
				console.PrintTableFunc(projects, []string{
					fmt.Sprintf("项目(%d)", len(projects)),
					"路径",
					"当前分支",
					"Master差异",
					"当前工作区是否干净",
				}, func(p *entities.Project) []string {
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
				})
			}
		}
	},
}

// cmd `project info`
var projectInfoCmd = &easycobra.Command{
	Use:   "info {project : 项目名，支持模糊匹配} {--w|workspace= : 指定工作区，默认针对所有工作区}",
	Short: "打开项目",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) {
		// init flags
		var workspaceName string
		cmd.Flags().StringVarP(&workspaceName, "workspace", "w", "", "指定工作区，默认针对所有工作区")

		// run
		return func(cmd *cobra.Command, args []string) {
			query := args[0]

			// 匹配项目
			proj := selectProject(query, workspaceName)
			if proj == nil {
				return
			}

			fmt.Printf("project: %s\n", proj.Name())
			fmt.Printf("path   : %s\n", proj.Path())
			fmt.Printf("git-url: %s\n", proj.RepoUrl())
		}
	},
}

// cmd `project open`
var projectOpenCmd = &easycobra.Command{
	Use:   "open {project : 项目名} {--app= : 打开项目的App} {--w|workspace= : 指定工作区，仅交互模式有意义}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) {
		// init flags
		var workspaceName string
		var appName string
		cmd.Flags().StringVarP(&workspaceName, "workspace", "w", "", "指定工作区，默认针对所有工作区")
		cmd.Flags().StringVar(&appName, "app", "", "打开项目的App")

		// run
		return func(cmd *cobra.Command, args []string) {
			query := args[0]

			// 获取打开项目的app
			applicationService := app.Default().ApplicationService()
			openApp := applicationService.FindByName(appName)
			if openApp == nil {
				log.Fatal("未找到指定app: " + appName)
				return
			}

			// 匹配项目
			proj := selectProject(query, workspaceName)
			if proj == nil {
				return
			}

			// 打开项目
			err := passthruRun(openApp.Bin(), proj.Path())
			if err != nil {
				log.Fatalf("打开失败: " + err.Error())
			}
		}
	},
}

func selectProject(query string, workspace string) *entities.Project {
	service := app.Default().ProjectService()
	projects := service.SearchInWorkspace(query, workspace)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, ok := console.ChoiceItem("选择项目", projects, (*entities.Project).Name)
		if !ok {
			fmt.Println("选择项目失败")
			return nil
		}
		return proj
	}
}

// cmd `project clone`
var projectCloneCmd = &easycobra.Command{
	Use:   "clone {repoUrl} {--depth= : 克隆深度，默认为不限制} {--b|branch=}",
	Short: "使用 RepoUrl 初始化项目",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) {
		// init flags
		var depth int
		var branch string
		cmd.Flags().IntVar(&depth, "depth", -1, "克隆深度，默认为不限制")
		cmd.Flags().StringVarP(&branch, "branch", "b", "", "分支名，默认为master")

		// run
		return func(cmd *cobra.Command, args []string) {
			rawRepoUrl := args[0]
			if branch != "" && depth == 0 {
				depth = 1 // // 指定分支情况下，默认深度为1
			}

			// 解析 repoUrl
			u, err := git.ParseRepoUrl(rawRepoUrl)
			if err != nil {
				log.Fatalf("repoUrl 不是合法地址: url=%s", rawRepoUrl)
				return
			}

			// 匹配hub
			remoteService := app.Default().RemoteService()
			remote := remoteService.FindByHost(u.Host)
			if remote == nil {
				log.Fatalf("repoUrl 没有对应 remote 配置: host=%s", u.Host)
				return
			}

			// 匹配本地地址
			localPath, ok := remote.MapDefaultPath(u)
			if !ok {
				log.Fatalf("对应 remote 未支持此路径: remote=%s, path=%s", remote.Name(), u.Path)
				return
			}

			// 执行命令
			err = passthruGitClone(localPath, rawRepoUrl, depth, branch)
			if err != nil {
				log.Fatalf("执行 clone 命令失败: " + err.Error())
			}
		}
	},
}

func passthruGitClone(localPath string, repoUrl string, depth int, branch string) error {
	args := []string{"git", "clone", repoUrl, localPath}
	if depth > 0 {
		args = append(args, "--depth="+strconv.Itoa(depth))
	}
	if branch != "" {
		args = append(args, "--branch="+branch)
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Println("Run Cmd >>> " + cmd.String())
	return cmd.Run()
}
