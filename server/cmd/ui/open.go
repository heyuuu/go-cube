package ui

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"cube/app"
	"cube/serve"
	"cube/util/pathkit"
)

// openPage ui 命令族共用的「路径 → 页面」流程：路径解析（~ / 相对 / 裸文件名补 ./，
// cwd 解析属出口层职责）→ 存在性校验 → server 探活 → 拼 路由+QueryEscape(path) 开浏览器。
// route 形如 "md" / "workbench"（前端路由段）。
func openPage(a *app.App, route string, rawPath string) error {
	if rawPath[0] != '/' && rawPath[0] != '~' && rawPath[0] != '.' {
		rawPath = "./" + rawPath
	}
	absPath, err := pathkit.AbsPath(rawPath)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}
	// 目录也放行：/md 页对目录展示左侧文件树（无 md 的目录显示空态）
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("路径不存在: %s", absPath)
	}

	if st := serve.Status(a.Server().Port()); !st.Running {
		return errors.New("server 未运行，请先执行: cube server start")
	}

	openInBrowser(fmt.Sprintf("%s%s?path=%s", a.Server().ServerURL(), route, url.QueryEscape(absPath)))
	return nil
}
