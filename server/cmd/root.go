package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/alfred"
	"cube/cmd/dev"
	"cube/cmd/server"
	"cube/config"
	"cube/logger"
	"cube/version"
)

func newRootCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cube",
		Short: "cube " + version.Version,
		Long: `cube —— 面向个人开发者的本地多项目管理工具（CLI 优先 + 本地 Web）。

命令按领域分组：
  - 项目：projects(列表) info(详情) open(打开) init/clone(初始化) check(检查)
  - opener：openers(列表) open-path(打开路径) diff(对比)
  - git：push(批量推送) pull(批量拉取) remote-status(分支与 remote 差距)
  - Web：server(本地服务) openapi(导出 API spec)

配置默认在 ~/.config/cube/，全局 flag -c 可覆盖配置目录，-d 开 debug 日志。`,
	}

	cmd.AddCommand(newVersionCmd(a))

	// web server 相关
	cmd.AddCommand(server.NewCmd(a))
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
	cmd.AddCommand(newPullCmd(a))
	cmd.AddCommand(newRemoteStatusCmd(a))

	// 内部命令
	cmd.AddCommand(alfred.NewCmd(a))
	cmd.AddCommand(dev.NewCmd(a))

	// 待整理命令
	cmd.AddCommand(newCheckCmd(a))

	return cmd
}

const defaultConfigPath = "~/.config/cube/config.json"

func Execute() {
	// 在 cobra 初始化之前，使用 Go 原生 flag 包预解析全局 flag（--config, --debug）
	cfgFile, debug, remaining := extractGlobalFlags(os.Args[1:], defaultConfigPath)

	// 初始化配置
	cfg, err := config.Load(cfgFile)
	checkError(err, "加载配置文件失败")

	// 尽量在其他行为前初始化 Logger
	logger.Init(cfg.Log, debug)
	slog.Info("初始化 logger", "debug", debug)

	// 初始化 App
	a, err := app.New(cfg)
	checkError(err, "App 初始化失败")

	// 构建 cmd
	cmd := newRootCmd(a)
	cmd.SetArgs(remaining)

	// cmd 上绑定全局 flag，仅用于生成 help 提示(此时 --config 及 --debug 早解析完了)
	cmd.PersistentFlags().String("config", defaultConfigPath, "config folder path (default is ~/.config/cube/config.json)")
	cmd.PersistentFlags().BoolP("debug", "D", false, "enable debug mode")

	// 执行命令
	err = cmd.Execute()
	checkError(err, "命令执行失败")
}

func checkError(err error, msg string) {
	if err != nil {
		slog.Error(msg, "err", err)
		// exit 前的错误信息，直接输出方便排查问题
		fmt.Printf("%s: %v", msg, err)
		os.Exit(1)
	}
}

// extractGlobalFlags 从 args 任意位置摘出 --debug / --config，返回 (cfgFile, debug, 剩余 args)。
// 不识别的 token（含子命令、子命令自己的 flag、位置参数）原样留在 remaining 里。
func extractGlobalFlags(args []string, defaultCfg string) (cfgFile string, debug bool, remaining []string) {
	cfgFile = defaultCfg
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--debug" || arg == "-D":
			debug = true
		case arg == "--config":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				cfgFile = args[i+1]
				i++
			}
		case strings.HasPrefix(arg, "--config="):
			cfgFile = strings.TrimPrefix(arg, "--config=")
		default:
			remaining = append(remaining, arg)
		}
	}
	return
}
