// Package project 提供 `cube project`（别名 proj/p）命令组，管理本地项目：
// 扫描识别、列表/搜索、打开、克隆、初始化、检查、目录树、git 缓存刷新。
//
// 各业务子命令按「一命令一文件」组织（list.go / info.go / open.go / clone.go 等），
// 本文件只放命令组入口 RootCmd 与跨命令共享的 helper。
package project

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/cmd/util/easycobra"
	"cube/cmd/util/tui"
	"cube/project"
)

// RootCmd 是 `cube project`（别名 proj/p）命令组入口，纯分发。
var RootCmd = &easycobra.Command{
	Use:     "project",
	Aliases: []string{"proj", "p"},
	Children: []*easycobra.Command{
		listCmd,
		infoCmd,
		openCmd,
		scanRulesCmd,
		cloneRulesCmd,
		cloneCmd,
		initCmd,
		refreshGitCacheCmd,
		checkCmd,
		treeCmd,
	},
}

// triggerAsyncRefresh 触发后台异步刷新（TTL 内会自动跳过，不阻塞当前命令）。
// 各子命令执行后通过 PersistentPostRun 统一调用（refresh-git-cache 除外）。
func triggerAsyncRefresh() {
	slog.Info("triggerAsyncRefresh")
	service := app.Default().ProjectService()
	service.TriggerAsyncRefresh()
}

func init() {
	RootCmd.CobraCommand().PersistentPostRun = func(cmd *cobra.Command, args []string) {
		if cmd != refreshGitCacheCmd.CobraCommand() {
			triggerAsyncRefresh()
		}
	}
}

// selectProject 按查询词匹配项目：0 个提示、1 个直接返回、多个交互选择。
// 供 list/info/open 等需要"定位单个项目"的命令复用。
func selectProject(query string) *project.Project {
	service := app.Default().ProjectService()
	projects := service.Search(query)
	switch len(projects) {
	case 0:
		fmt.Println("没有匹配的项目")
		return nil
	case 1:
		return projects[0]
	default:
		proj, err := tui.SelectItem("选择项目", projects, (*project.Project).Name)
		if err != nil {
			fmt.Printf("选择项目失败: %v\n", err)
			return nil
		}
		return proj
	}
}
