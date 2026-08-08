package project

import (
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"cube/app"
	"cube/config"
	"cube/project/gitcache"
)

// cmd `project refresh-git-cache`
//
// 在子进程里同步执行一次 git 信息缓存采集。通常不直接由用户调用 ——
// 而是由 `project list` / `project info` 在返回前通过 gitcache.TryAsyncRefresh
// fork 本命令到后台执行；但保留给用户手动触发的能力（强制刷新）。
//
// 行为（与 TryAsyncRefresh 的双保险一致）：
//   - flock(git.lock, LOCK_EX|LOCK_NB)：拿不到锁立即退出（已有进程在跑）
//   - 拿到锁后立即写 lastRefreshAt 占住 TTL 窗口
//   - Load 缓存 → Refresh 采集 → 落盘
//
// 错误只 slog 记录，绝不 panic —— 即便作为后台子进程也不应影响父进程。

func newRefreshGitCacheCmd(a *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh-git-cache",
		Short: "刷新 git 信息缓存",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cacheDir := filepath.Join(config.Path(), "cache")
			projects := a.ProjectService().Projects()
			paths := make([]string, 0, len(projects))
			for _, p := range projects {
				paths = append(paths, p.Path())
			}

			if err := gitcache.RefreshSync(cacheDir, paths); err != nil {
				slog.Warn("refresh-git-cache failed", "err", err, "projects", len(paths))
				return err
			}
			slog.Debug("refresh-git-cache done", "projects", len(paths))
			return nil
		},
	}
	return cmd
}
