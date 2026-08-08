package cmd

import (
	"flag"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/alfred"
	"cube/cmd/dev"
	"cube/cmd/gitx"
	"cube/cmd/opener"
	"cube/cmd/project"
	"cube/cmd/server"
	"cube/cmd/ugly"
	"cube/cmd/ui"
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
	cmd.AddCommand(alfred.NewCmd(a))
	cmd.AddCommand(server.NewCommand(a))
	cmd.AddCommand(ui.NewCommand(a))
	cmd.AddCommand(project.NewCommand(a))
	cmd.AddCommand(opener.NewCommand(a))
	cmd.AddCommand(ugly.NewCommand(a))
	cmd.AddCommand(dev.NewCommand(a))
	cmd.AddCommand(gitx.NewCommand(a))

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

	// 设置 debug 环境
	config.SetDebug(debug)

	// 初始化配置
	cfg, err := config.Load(cfgFile)
	checkError(err, "加载配置文件失败")

	// 尽量在其他行为前初始化 Logger
	logger.Init(cfg.Log)

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
