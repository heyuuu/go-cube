package dev

import (
	"iter"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"cube/app"
	"cube/util/tui"
)

// cmd `cube dev commands`
//
// 收集所有可用命令，按字典序排列后以表格形式输出。
func newCommandsCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "列出所有命令",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := findRoot(cmd)
			all := slices.Collect(iterCommands(root, ""))
			sort.Slice(all, func(i, j int) bool {
				return all[i].path < all[j].path
			})

			headers := []string{"命令", "别名", "说明", "标志/参数"}
			rows := make([][]string, len(all))
			for i, e := range all {
				rows[i] = []string{e.path, e.aliases, e.short, e.flags}
			}
			tui.PrintTable(headers, rows)
			return nil
		},
	}
}

// cmdEntry 收集一个命令的关键信息。
type cmdEntry struct {
	path    string // 完整命令路径，如 "cube project list"
	aliases string // 逗号拼接的别名
	short   string // Short 描述
	flags   string // 关键本地标志/参数
}

// findRoot 沿 Parent() 链向上找到根命令。
func findRoot(cmd *cobra.Command) *cobra.Command {
	for cmd.Parent() != nil {
		cmd = cmd.Parent()
	}
	return cmd
}

// collectCommands 遍历命令树中所有可用命令的信息。
func iterCommands(cmd *cobra.Command, prefix string) iter.Seq[cmdEntry] {
	return func(yield func(cmdEntry) bool) {
		for _, c := range cmd.Commands() {
			if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
				continue
			}
			// 跳过 cobra 自动生成的内置命令（completion、help 等）
			if isBuiltinCommand(c) {
				continue
			}
			name := c.Name()
			if prefix != "" {
				name = prefix + " " + name
			}

			// 只收集有执行函数的命令，不收集纯分组命令
			if c.Run != nil || c.RunE != nil {
				yield(cmdEntry{
					path:    name,
					aliases: joinAliases(c.Aliases),
					short:   c.Short,
					flags:   collectFlags(c),
				})
			}

			// 递归收集其子命令
			for child := range iterCommands(c, name) {
				if !yield(child) {
					return
				}
			}
		}
	}
}

// joinAliases 将别名切片拼接为逗号分隔的字符串。
func joinAliases(aliases []string) string {
	return strings.Join(aliases, ", ")
}

// collectFlags 收集命令的本地非隐藏标志和位置参数，返回简短摘要。
func collectFlags(cmd *cobra.Command) string {
	var parts []string
	// 本地标志
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		var s string
		if flag.Shorthand != "" {
			s = "-" + flag.Shorthand + ", --" + flag.Name
		} else {
			s = "--" + flag.Name
		}
		parts = append(parts, s)
	})
	// 位置参数
	if cmd.Args != nil {
		parts = append(parts, "[args]")
	}
	return strings.Join(parts, " ")
}

// isBuiltinCommand 判断是否为 cobra 自动生成的内置命令（completion、help 等）。
// cobra 内置命令没有分组注解 tag，且名称属于已知内置集合。
func isBuiltinCommand(cmd *cobra.Command) bool {
	name := cmd.Name()
	return name == "completion" || name == "help"
}
