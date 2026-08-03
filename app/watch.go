package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/heyuuu/cube/config"
)

// Reload 重新加载磁盘配置并应用到各 service。
// 供配置监听器在 config.json 变更后调用，让长驻进程无需重启即可应用新配置。
//
// 当前 reload 的范围：project.Service（scan/clone 规则）+ opener.Service（openers 列表）。
// web.Server 的端口/路由不动（路由定义稳定，无需重建）。
// db/history 不 reload（数据层与配置无关）。
func (app *App) Reload() {
	// 重新解析磁盘 config.json 覆盖 config 包的 defaultConf
	cfgFile := config.ConfigFile()
	if err := config.Init(config.Path()); err != nil {
		slog.Warn("reload: 重新加载 config 失败", "file", cfgFile, "err", err)
		return
	}
	conf := config.Default()

	// 应用到各 service（各 service 内部有自己的锁）
	app.projectService.Reload(conf.Project)
	app.openerService.Reload(conf)
	slog.Info("config reloaded", "file", cfgFile)
}

// WatchConfig 监听 config.json 变更，触发 Reload。
// 阻塞运行直到 ctx 取消。内部对 fsnotify 事件做去抖（避免编辑器保存时的连续事件）。
//
// 监听配置目录而非文件本身：编辑器（尤其 vim）保存常用「写临时文件 + rename」原子替换，
// 直接 watch 文件会丢失 rename 后的事件，watch 目录则能捕获。
func (app *App) WatchConfig(ctx context.Context) error {
	cfgFile := config.ConfigFile()
	cfgDir := filepath.Dir(cfgFile)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Add(cfgDir); err != nil {
		return err
	}
	slog.Info("watching config for changes", "dir", cfgDir, "file", cfgFile)

	// 去抖：保存触发的连续 Write/Rename/Create 事件合并为一次 reload
	const debounce = 300 * time.Millisecond
	var (
		mu      sync.Mutex
		timer   *time.Timer
		pending = false
	)
	triggerReload := func() {
		mu.Lock()
		defer mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		pending = true
		timer = time.AfterFunc(debounce, func() {
			mu.Lock()
			isPending := pending
			pending = false
			mu.Unlock()
			if isPending {
				app.Reload()
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// 只关心 config.json 相关事件
			if event.Name != cfgFile {
				continue
			}
			slog.Debug("config file event", "op", event.Op, "name", event.Name)
			triggerReload()
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			slog.Warn("config watcher error", "err", err)
		}
	}
}
