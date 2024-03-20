package cmd

import (
	"go-cube/internal/config"
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
		return config.InitConfigFile(cfgFile)
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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default is $HOME/.go-cube.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "open debug mode")
}
