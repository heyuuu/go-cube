package cmd

import (
	"fmt"

	"cube/cmd/util/easycobra"
	"cube/version"
)

// versionCmd represents the version command
var versionCmd = &easycobra.Command{
	Use:   "version",
	Short: "show version",
	Run: func(args []string) error {
		fmt.Println("cube " + version.Version)
		return nil
	},
}
