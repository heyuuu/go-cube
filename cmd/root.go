package cmd

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/heyuuu/cube/cmd/alfred"
	cmdConfig "github.com/heyuuu/cube/cmd/config"
	"github.com/heyuuu/cube/cmd/opener"
	"github.com/heyuuu/cube/cmd/project"
	"github.com/heyuuu/cube/cmd/server"
	"github.com/heyuuu/cube/cmd/util/easycobra"
	"github.com/heyuuu/cube/config"
	"github.com/heyuuu/cube/db"
	"github.com/heyuuu/cube/history"
	"github.com/heyuuu/cube/logger"
	"github.com/heyuuu/cube/version"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &easycobra.Command{
	Use:   "cube",
	Short: "cube " + version.Version,
	Children: []*easycobra.Command{
		// group commands
		alfred.RootCmd,
		server.RootCmd,
		project.RootCmd,
		opener.RootCmd,
		cmdConfig.RootCmd,
		// simple commands
		versionCmd,
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cmd := rootCmd.CobraCommand()

	// persistent flags
	var cfgPath string
	var debug bool
	cmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "config folder path (default is ~/.config/cube/)")
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "open debug mode")

	// 初始化：在 cobra 解析完 flag 后、命令执行前触发。
	// 用 OnInitialize 而非 PersistentPreRunE：后者只在最终命中的 runnable 命令上触发，
	// 而本项目 cube/project 等是纯分组命令（无 Run），不会触发 PersistentPreRunE；
	// OnInitialize 在任意命令执行前都会可靠触发。
	cobra.OnInitialize(func() {
		// 设置 debug 环境
		config.SetDebug(debug)

		// 初始化配置
		err := config.Init(cfgPath)
		checkError(err, "init config failed")

		// 初始化 Logger
		logger.Init()

		// 初始化 DB
		err = db.Init(config.Path(),
			&history.ProjectSelectLog{},
			&history.ProjectOpenLog{},
		)
		checkError(err, "init db failed")

		// 记录启动日志
		slog.Debug("command start", "debug", debug, "cfgPath", config.Path(), "args", os.Args)
	})

	// 执行命令
	err := rootCmd.Execute()
	checkError(err, "execute failed")
}

func checkError(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
