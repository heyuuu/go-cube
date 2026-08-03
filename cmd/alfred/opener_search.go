package alfred

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/app"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/opener"
	"github.com/heyuuu/cube/util/slicekit"
)

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
