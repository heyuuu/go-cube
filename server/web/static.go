package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
)

// uiFS 前端静态资源
//
//go:embed ui/*
var uiFS embed.FS

// registerStaticRoutes 挂载前端静态资源路由：
//   - GET /        → ui/index.html（首页入口）
//   - GET /ui/*    → ui/ 下其它静态文件（css/js/vendor）
//
// 资源来自 main 注入的 uiAssets（//go:embed ui）。为空 FS 时跳过挂载。
func registerStaticRoutes(mux *http.ServeMux) {
	// 判断是否有 index.html 判断资源是否正常
	indexFile, err := fs.ReadFile(uiFS, "ui/index.html")
	if err != nil {
		slog.Error("ui assets 为空，无法挂载静态资源", "err", err)
		return
	}

	// /ui/* —— 文件服务器喂 ui/ 子树
	fileServer := http.FileServer(http.FS(uiFS))
	mux.Handle("/ui/", fileServer)

	// / —— 首页（cube 当前无前端路由，根路径直接返回 index.html）
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 标准库 mux 的 "/" 是 catch-all 兜底；只对真正的根路径与未匹配的非 api 路径回 index
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(indexFile)
			return
		}
		http.NotFound(w, r)
	})
}
