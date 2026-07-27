package project

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/cmd/util/runner"
	"github.com/heyuuu/cube/cmd/util/tui"
	"github.com/heyuuu/cube/project"
	"github.com/heyuuu/cube/util/git"
	"github.com/heyuuu/cube/util/pathkit"
	"github.com/heyuuu/cube/util/slicekit"
)

// cmd group `project *`
var RootCmd = &easycobra.Command{
	Use:     "project",
	Aliases: []string{"proj", "p"},
	Children: []*easycobra.Command{
		projectListCmd,
		projectInfoCmd,
		projectOpenCmd,
		projectScanRulesCmd,
		projectCloneRulesCmd,
		projectCloneCmd,
		projectInitCmd,
		projectRefreshGitCacheCmd,
		projectCheckCmd,
		projectTreeCmd,
	},
}

// 触发后台异步刷新（TTL 内会自动跳过，不阻塞当前命令）
func triggerAsyncRefresh() {
	slog.Info("triggerAsyncRefresh")
	service := app.Default().ProjectService()
	service.TriggerAsyncRefresh()
}

func init() {
	RootCmd.CobraCommand().PersistentPostRun = func(cmd *cobra.Command, args []string) {
		if cmd != projectRefreshGitCacheCmd.CobraCommand() {
			triggerAsyncRefresh()
		}
	}
}

// cmd `project list`
var projectListCmd = &easycobra.Command{
	Use:   "list [query]",
	Short: "项目列表(支持模糊搜索)",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var verbose int
		var group string
		cmd.Flags().CountVarP(&verbose, "verbose", "v", "verbose level (-v, -vv, -vvv)")
		cmd.Flags().StringVarP(&group, "group", "g", "", "限定分组 group")

		// run
		return func(args []string) error {
			// 获取输入参数
			var query string
			if len(args) > 0 {
				query = args[0]
			}

			// 项目列表
			service := app.Default().ProjectService()
			projects := service.Search(query)

			// 按 group 过滤
			if len(group) > 0 {
				projects = slices.DeleteFunc(projects, func(p *project.Project) bool {
					return p.Group() != group
				})
			}

			// 展示项目列表
			showProjects(projects, verbose)

			return nil
		}
	},
}

func showProjects(projects []*project.Project, verbose int) {
	var headers []string
	rows := make([][]string, len(projects))

	// verbose: 0
	headers = append(headers, fmt.Sprintf("项目(%d)", len(projects)), "Path", "RepoUrl")
	for i, p := range projects {
		rows[i] = append(rows[i], p.Name(), pathkit.PrettyPath(p.Path()), p.RepoUrl())
	}

	// verbose: 1
	if verbose >= 1 {
		headers = append(headers, "当前分支", "默认分支", "默认分支差异", "当前工作区是否干净")
		for i, p := range projects {
			info := p.GitInfo()

			var currBranch, defaultBranch, branchDiff, statusText string
			if info != nil {
				currBranch = info.CurrentBranch
				defaultBranch = info.DefaultBranch
				if info.Ahead != 0 {
					branchDiff += "+" + strconv.Itoa(info.Ahead)
				}
				if info.Behind != 0 {
					branchDiff += "-" + strconv.Itoa(info.Behind)
				}
				if info.Dirty {
					statusText = "dirty"
				}
			}

			rows[i] = append(rows[i], currBranch, defaultBranch, branchDiff, statusText)
		}
	}

	// 输出表格
	tui.PrintTable(headers, rows)
}

// cmd `project info`
var projectInfoCmd = &easycobra.Command{
	Use:   "info [query]",
	Short: "打开项目(支持模糊搜索)",
	Args:  cobra.ExactArgs(1),
	Run: func(args []string) error {
		query := args[0]

		// 匹配项目
		proj := selectProject(query)
		if proj == nil {
			return nil
		}

		fmt.Printf("project: %s\n", proj.Name())
		fmt.Printf("path   : %s\n", proj.Path())
		// repoUrl 读 git 缓存（Project.RepoUrl 字段已废弃）
		var repoUrl string
		if info, ok := app.Default().ProjectService().GitInfo(proj.Path()); ok {
			repoUrl = info.RepoUrl
		}
		fmt.Printf("git-url: %s\n", repoUrl)

		return nil
	},
}

// cmd `project open`
var projectOpenCmd = &easycobra.Command{
	Use:   "open {project : 项目名} {--app= : 打开项目的App}",
	Short: "打开项目。非交互模式只支持准确项目名，非交互模式下支持模糊搜索",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		var appName string
		cmd.Flags().StringVar(&appName, "app", "", "打开项目的App")

		// run
		return func(args []string) error {
			query := args[0]

			// 获取打开项目的app
			openerService := app.Default().OpenerService()
			openApp := openerService.FindByName(appName)
			if openApp == nil {
				return errors.New("未找到指定app: " + appName)
			}

			// 匹配项目
			proj := selectProject(query)
			if proj == nil {
				return nil
			}

			// 打开项目
			err := runner.Run(openApp.Bin(), proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		}
	},
}

func selectProject(query string) *project.Project {
	service := app.Default().ProjectService()
	projects := service.Search(query)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, err := tui.SelectItem("选择项目", projects, (*project.Project).Name)
		if err != nil {
			fmt.Printf("选择项目失败: %v\n", err)
			return nil
		}
		return proj
	}
}

// cmd `project scan-rules`
var projectScanRulesCmd = &easycobra.Command{
	Use:   "scan-rules",
	Short: "列出 scan 规则",
	Run: func(args []string) error {
		service := app.Default().ProjectService()
		for _, rule := range service.ScanRules() {
			fmt.Println(rule.Group)
		}
		return nil
	},
}

// cmd `project clone-rules`
var projectCloneRulesCmd = &easycobra.Command{
	Use:   "clone-rules",
	Short: "列出 clone 规则",
	Run: func(args []string) error {
		service := app.Default().ProjectService()
		rules := service.CloneRules()

		// 显示列表
		tui.PrintTable(
			[]string{
				fmt.Sprintf("RepoHost(%d)", len(rules)),
				"RepoPrefix",
				"LocalPath",
			},
			slicekit.Map(rules, func(r project.CloneRule) []string {
				return []string{
					r.RepoHost,
					r.RepoPrefix,
					r.LocalPath,
				}
			}),
		)
		return nil
	},
}

// cmd `project clone`
var projectCloneCmd = &easycobra.Command{
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
