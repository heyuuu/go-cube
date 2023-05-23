package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// projectCmd represents the project command
var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "A brief description",
	Long:  `A longer description`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("project called")
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
}
