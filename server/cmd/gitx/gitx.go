// Package gitx 提供 git 增强命令，封装「单仓库 × 多 remote × 多 ref」的批量操作。
//
// 适用场景：个人仓库常常配置了多个 remote（origin / 镜像 / 备份等），
// 这些 remote 上的提交内容是同步的，但需要逐个 push 才能保持一致。
// 本包把这类「跨 remote 批量动作」做成交互式 TUI 命令，免去手敲多条 git push。
//
// 本包只做分发与编排：git 读写复用 util/gogit（读）与 util/git（写），
// 交互复用 cmd/util/tui，不在此处重复造轮子。
package gitx

import (
	"github.com/spf13/cobra"

	"cube/app"
)

// RootCmd 是 `cube gitx`（别名 `gx`）命令组入口，纯分发。

func NewCommand(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "gitx",
		Aliases: []string{"gx"},
		Short:   "git 增强：多 remote 批量 push 等跨仓库常用动作",
	}
	cmd.AddCommand(newPushCmd(a))
	cmd.AddCommand(newRemoteStatusCmd(a))
	return cmd
}
