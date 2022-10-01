/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"fmt"
	"go-cube/internal/project"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// projectSearchCmd represents the projectSearch command
var projectSearchCmd = &cobra.Command{
	Use:   "project:search",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		manager := project.DefaultManager()
		projects := manager.Projects()

		sort.Slice(projects, func(i, j int) bool {
			return projects[i].Name < projects[j].Name
		})

		var maxNameLen int
		for _, proj := range projects {
			if maxNameLen < len(proj.Name) {
				maxNameLen = len(proj.Name)
			}
		}

		for index, proj := range projects {
			fmt.Printf("[%3d] %s %s\n", index, strRightPad(proj.Name, maxNameLen), proj.Path)
		}
	},
}

func init() {
	rootCmd.AddCommand(projectSearchCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// projectSearchCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// projectSearchCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func strRightPad(s string, minLen int) string {
	if len(s) >= minLen {
		return s
	} else {
		return s + strings.Repeat(" ", minLen-len(s))
	}
}
