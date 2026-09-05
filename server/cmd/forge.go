package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/forge"
	"cube/util/iconkit"
	"cube/util/slicekit"
	"cube/util/tui"
)

// newForgeCmd forge 子命令组（git 托管平台配置管理，提案 1040）。
// 裸跑 `cube forge` = `cube forge list`（查看比管理高频）。
func newForgeCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forge",
		Short: "git 托管平台（forge）配置管理",
		Long: `管理 forge 配置（git 托管平台实例，host 级一条）：
域名（如 github.com、自建 gitea.example.com）、API 方言 kind、图标。
数据存 settings.json 的 forges 节，增删改也可在 Web 设置页完成。`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runForgeList(a)
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "列出全部 forge",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runForgeList(a)
		},
	})
	return cmd
}

func runForgeList(a *app.App) error {
	forges := a.ForgeService().Forges()
	tui.PrintTable(
		[]string{
			fmt.Sprintf("Forge(%d)", len(forges)),
			"Kind",
			"Icon",
		},
		slicekit.Map(forges, func(f forge.Forge) []string {
			return []string{f.Host, f.Kind, iconSummary(f.Icon)}
		}),
	)
	return nil
}

// iconSummary CLI 侧 icon 概要：lucide 显图名，image 只显标记（base64 不进表格）。
func iconSummary(icon *iconkit.Icon) string {
	if icon == nil {
		return "-"
	}
	if icon.Type == iconkit.IconTypeImage {
		return "[image]"
	}
	return icon.Value
}
