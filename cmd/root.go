package cmd

import (
	"github.com/heyuuu/go-cube/cmd/alfred"
	"github.com/heyuuu/go-cube/cmd/application"
	"github.com/heyuuu/go-cube/cmd/project"
	"github.com/heyuuu/go-cube/cmd/remote"
	"github.com/heyuuu/go-cube/cmd/workspace"
	"github.com/heyuuu/go-cube/internal/config"
	"github.com/heyuuu/go-cube/internal/cube"
	"github.com/heyuuu/go-cube/internal/util/easycobra"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &easycobra.Command{
	Use:   "go-cube",
	Short: "go-cube " + cube.Version,
	InitPersistentPreRunE: func(cmd *cobra.Command) func(cmd *cobra.Command, args []string) error {
		// persistent flags
		var cfgPath string
		var debug bool
		cmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config folder path (default is ~/.go-cube/)")
		cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "open debug mode")

		// persistent pre run
		return func(cmd *cobra.Command, args []string) (err error) {
			// 设置 debug 环境
			config.SetDebug(debug)

			// 初始化配置
			err = config.InitConfig(cfgPath)
			if err != nil {
				return err
			}

			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(
		// group commands
		project.ProjectCmd,
		application.AppCmd,
		remote.RemoteCmd,
		workspace.WorkspaceCmd,
		alfred.AlfredCmd,
		// simple commands
		versionCmd,
		serverCmd,
	)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.CobraCommand().Execute()
	if err != nil {
		os.Exit(1)
	}
}
