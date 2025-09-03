package cmd

import (
	"github.com/heyuuu/go-cube/cmd/alfred"
	"github.com/heyuuu/go-cube/cmd/application"
	"github.com/heyuuu/go-cube/cmd/project"
	"github.com/heyuuu/go-cube/cmd/remote"
	"github.com/heyuuu/go-cube/cmd/workspace"
	"github.com/heyuuu/go-cube/internal/config"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string
var debug bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "go-cube",
	Short: "go-cube v0.2.0",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		config.SetDebug(debug)
		return config.InitConfig(cfgFile)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is ~/.go-cube/config.json)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "open debug mode")

	rootCmd.AddCommand(project.ProjectCmd.CobraCommand())
	rootCmd.AddCommand(application.AppCmd.CobraCommand())
	rootCmd.AddCommand(remote.RemoteCmd.CobraCommand())
	rootCmd.AddCommand(workspace.WorkspaceCmd.CobraCommand())
	rootCmd.AddCommand(alfred.AlfredCmd.CobraCommand())
}
