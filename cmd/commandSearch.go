package cmd

import (
	"fmt"
	"go-cube/internal/command"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// commandSearchCmd represents the commandSearch command
var commandSearchCmd = &cobra.Command{
	Use:   "command:search [query]*",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		var commands []command.Command
		if len(args) == 0 {
			commands = command.DefaultManager().Commands()
			sort.Slice(commands, func(i, j int) bool {
				return commands[i].Name < commands[j].Name
			})
		} else {
			query := strings.Join(args, " ")
			commands = command.DefaultManager().Search(query)
		}

		// 返回结果
		if alfred {
			var items []any
			for _, cmd := range commands {
				items = append(items, map[string]string{
					"title":    cmd.Name,
					"subtitle": cmd.Bin,
					"arg":      cmd.Name,
				})
			}
			alfredSearchResult(items)
		} else {
			// 展示数据
			for index, cmd := range commands {
				fmt.Printf("[%3d] %s %s\n", index, cmd.Name, cmd.Bin)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(commandSearchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// commandSearchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// commandSearchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
