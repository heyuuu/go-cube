package dev

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"cube/app"
)

// cmd `cube dev refresh-gitcache`
//
// 手动触发一次 gitcache 全量采集：重扫项目列表 → 并发采集全部项目的
// git 信息 → 覆盖写 ~/.config/cube/cache/git.json。
//
// CLI 平时只读缓存不写（单写者模型：server 是唯一写方），本命令是刻意的
// 例外，用于开发期实测全量采集的时间成本、验证采集结果。落盘走原子
// rename，与正在运行的 server 并发写也不会产生损坏文件（至多相互覆盖）。
func newRefreshGitCacheCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh-gitcache",
		Short: "手动触发 gitcache 全量采集（重扫项目 + 刷新 git.json，实测耗时）",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("开始采集全部项目 git 信息 ...")
			start := time.Now()
			total, collected, err := a.ProjectService().Refresh()
			cost := time.Since(start).Round(time.Millisecond)
			if err != nil {
				return fmt.Errorf("采集失败: %w（项目 %d / 成功 %d / 耗时 %s）", err, total, collected, cost)
			}
			fmt.Printf("采集完成：%d 个项目，成功 %d，失败 %d，耗时 %s\n",
				total, collected, total-collected, cost)
			return nil
		},
	}
}
