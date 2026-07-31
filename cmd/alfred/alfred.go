package alfred

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/project"
	"github.com/heyuuu/cube/util/slicekit"
)

var RootCmd = &easycobra.Command{
	Use: "alfred",
	Children: []*easycobra.Command{
		projectSearchCmd,
		projectOpenCmd,
		openerSearchCmd,
	},
}

// cmd `alfred project-search`
var projectSearchCmd = &easycobra.Command{
	Use:   "project-search {query?* : 项目名，支持模糊匹配}",
	Short: "搜索项目列表",
	Run: func(args []string) error {
		// 获取输入参数
		query := strings.Join(args, " ")

		// 项目列表
		service := app.Default().ProjectService()
		projects := service.Search(query)

		// 最近打开日志
		historyService := app.Default().HistoryService()
		history := historyService.LeastSelectedProjects(10, true)
		sortProjectsWithHistory(projects, history)

		// 返回结果
		return PrintResult(projects, func(proj *project.Project) Item {
			return Item{
				Title:    proj.Name(),
				SubTitle: proj.Path(),
				Arg:      proj.Name(),
			}
		})
	},
}

// 优先将 history 排在前面，保持其他顺序不变
func sortProjectsWithHistory(projects []*project.Project, history []string) []*project.Project {
	weights := make(map[string]int, len(history))
	for i, proj := range projects {
		weights[proj.Name()] = i + len(history)
	}
	for i, proj := range history {
		weights[proj] = i
	}

	slices.SortFunc(projects, func(a, b *project.Project) int {
		return weights[a.Name()] - weights[b.Name()]
	})

	return projects
}

// cmd `alfred opener-search`
var openerSearchCmd = &easycobra.Command{
	Use:   "opener-search [query]",
	Short: "搜索可用命令列表",
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var projectName string
		cmd.Flags().StringVar(&projectName, "project", "", "项目名")

		// run
		return func(args []string) error {
			query := args

			// sticky: alfred 选择项目后会以空参数调用此命令
			if len(query) == 0 && len(projectName) > 0 {
				app.Default().HistoryService().AddProjectSelectLog(projectName, true)
			}

			// 获取匹配的命令列表
			service := app.Default().OpenerService()
			openers := service.SearchFor(opener.RoleOpenDir, strings.Join(query, " "))

			// 若指定项目，且对应空间有指定命令优先级，则按优先级排序
			if len(projectName) > 0 {
				historyService := app.Default().HistoryService()
				history := historyService.LeastProjectOpenApps(projectName, 3, true)
				openers = sortOpeners(openers, history)
			}

			// 返回结果
			return PrintResult(openers, func(item *opener.Opener) Item {
				return Item{
					Title:    item.Name(),
					SubTitle: item.CmdString(),
					Arg:      item.Name(),
				}
			})
		}
	},
}

func sortOpeners(openers []*opener.Opener, history []string) []*opener.Opener {
	if len(openers) <= 1 || len(history) == 0 {
		return openers
	}

	type target struct {
		index  int
		weight int
	}

	historyMap := make(map[string]int, len(history))
	for i, openerName := range history {
		historyMap[openerName] = i
	}

	targets := make([]*target, len(openers))
	for i, o := range openers {
		if historyIdx, ok := historyMap[o.Name()]; ok {
			targets[i] = &target{index: i, weight: historyIdx}
		} else {
			targets[i] = &target{index: i, weight: i + len(history)}
		}
	}
	slices.SortFunc(targets, func(t1, t2 *target) int {
		return t1.weight - t2.weight
	})
	return slicekit.Map(targets, func(t *target) *opener.Opener {
		return openers[t.index]
	})
}

// cmd `alfred project-open`
var projectOpenCmd = &easycobra.Command{
	Use:   "project-open <project-name>",
	Short: "打开项目, 只支持准确项目绝对路径",
	Args:  cobra.ExactArgs(1),
	InitRun: func(cmd *cobra.Command) easycobra.Run {
		// init flags
		var openerName string
		cmd.Flags().StringVarP(&openerName, "opener", "o", "", "打开项目的 Opener")

		// run
		return func(args []string) error {
			projectName := args[0]

			// history: 记录打开项目的程序
			app.Default().HistoryService().AddProjectOpenLog(projectName, openerName, true)

			// 匹配项目
			projService := app.Default().ProjectService()
			proj := projService.FindByName(projectName)
			if proj == nil {
				return errors.New("未找到指定项目: " + projectName)
			}

			// 获取打开项目的app
			openerService := app.Default().OpenerService()
			opener := openerService.FindByName(openerName)
			if opener == nil {
				return errors.New("未找到指定 opener: " + openerName)
			}

			// 打开项目
			err := opener.Open(proj.Path())
			if err != nil {
				return fmt.Errorf("打开失败: %w", err)
			}

			return nil
		}
	},
}
