package cmd

import (
	"flag"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/alfred"
	"cube/cmd/dev"
	"cube/config"
	"cube/logger"
	"cube/version"
)

func newRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cube",
		Short: "cube " + version.Version,
	}

	cmd.AddCommand(newVersionCmd(a))
	// web server 相关
	cmd.AddCommand(newServerCmd(a))
	cmd.AddCommand(newOpenapiCmd(a))
	// project 相关
	cmd.AddCommand(newProjectsCmd(a)) // 项目列表
	cmd.AddCommand(newInfoCmd(a))     // 项目信息
	cmd.AddCommand(newOpenCmd(a))     // 打开项目
	cmd.AddCommand(newInitCmd(a))     // 初始化空项目
	//cmd.AddCommand(newCreateCmd(a))   // 使用模板初始化项目
	cmd.AddCommand(newCloneCmd(a)) // 使用 RepoUrl 初始化项目

	// open 相关
	cmd.AddCommand(newOpenersCmd(a))
	cmd.AddCommand(newOpenPathCmd(a))
	cmd.AddCommand(newDiffCmd(a))

	// git 相关
	cmd.AddCommand(newPushCmd(a))
	cmd.AddCommand(newRemoteStatusCmd(a))

	// 内部命令
	cmd.AddCommand(alfred.NewCmd(a))
	cmd.AddCommand(dev.NewCommand(a))

	// 待整理命令
	cmd.AddCommand(newCheckCmd(a))
	cmd.AddCommand(newRefreshGitCacheCmd(a))

	return cmd
}

const defaultConfigPath = "~/.config/cube/config.json"

func Execute() {
	// 在 cobra 初始化之前，使用 Go 原生 flag 包预解析全局 flag（--config, --debug）
	var cfgFile string
	var debug bool

	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.Usage = func() {} // 抑制未知 flag 的 usage 输出
	fs.StringVar(&cfgFile, "config", defaultConfigPath, "config folder path (default is ~/.config/cube/config.json)")
	fs.BoolVar(&debug, "debug", false, "enable debug mode")

	// 初始化配置
	cfg, err := config.Load(cfgFile)
	checkError(err, "加载配置文件失败")

	// 尽量在其他行为前初始化 Logger
	logger.Init(cfg.Log, debug)

	// 初始化 App
	a, err := app.New(cfg)
	checkError(err, "App 初始化失败")

	// 构建 cmd
	cmd := newRootCmd(a)

	// 执行命令
	err = cmd.Execute()
	checkError(err, "execute failed")
}

func checkError(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		os.Exit(1)
	}
}
