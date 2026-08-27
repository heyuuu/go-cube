package alfred

import (
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/opener"
	"cube/util/slicekit"
)

// cmd `alfred opener-search`
func newOpenerSearchCmd(a *app.App) *cobra.Command {
	var projectPath string
	cmd := &cobra.Command{
		Use:   "opener-search [query]",
		Short: "搜索可用命令列表",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args

			// 获取匹配的命令列表
			openers := a.OpenerService().SearchFor(opener.RoleOpenDir, strings.Join(query, " "))

			// 若指定项目（路径为项目唯一标识口径），且该项目有 opener 使用偏好，则按最近使用排序
			if len(projectPath) > 0 {
				history := a.UsageService().LatestOpeners(projectPath, 3)
				openers = sortOpeners(openers, history)
			}

			// 返回结果
			return PrintResult(openers, func(item opener.Opener) Item {
				return Item{
					Title:    item.Name(),
					SubTitle: item.Summary(),
					Arg:      item.Name(),
				}
			})
		},
	}

	cmd.Flags().StringVar(&projectPath, "project", "", "项目绝对路径")
	return cmd
}

func sortOpeners(openers []opener.Opener, history []string) []opener.Opener {
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
	return slicekit.Map(targets, func(t *target) opener.Opener {
		return openers[t.index]
	})
}
