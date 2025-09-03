package cmd

import (
	"fmt"
	"github.com/heyuuu/go-cube/internal/cube"
	"github.com/heyuuu/go-cube/internal/util/easycobra"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &easycobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("go-cube " + cube.Version)
	},
}
