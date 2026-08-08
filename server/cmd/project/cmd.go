// Package project 提供 `cube project`（别名 proj/p）命令组，管理本地项目：
// 扫描识别、列表/搜索、打开、克隆、初始化、检查、目录树、git 缓存刷新。
package project

import (
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"cube/app"
)

// RootCmd 是 `cube project`（别名 proj/p）命令组入口，纯分发。

func NewCommand(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"proj", "p"},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			// 触发后台异步刷新（TTL 内会自动跳过，不阻塞当前命令）。
			// 各子命令执行后通过 PersistentPostRun 统一调用（refresh-git-cache 除外）。
			if !strings.HasPrefix(cmd.Use, "refresh-git-cache") {
				slog.Info("triggerAsyncRefresh")
				a.ProjectService().TriggerAsyncRefresh()
			}
		},
	}

	cmd.AddCommand(newListCmd(a))
	cmd.AddCommand(newInfoCmd(a))
	cmd.AddCommand(newOpenCmd(a))
	cmd.AddCommand(newCloneCmd(a))
	cmd.AddCommand(newInitCmd(a))
	cmd.AddCommand(newCheckCmd(a))
	cmd.AddCommand(newRefreshGitCacheCmd(a))
	return cmd
}
