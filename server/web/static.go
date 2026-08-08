package web

import (
	"io/fs"
	"net/http"
)

// uiRoot embed 的 ui/ 子树根，注册时用。ui/ 下文件路径相对它解析。
const uiRoot = "ui"

// registerStaticRoutes 挂载前端静态资源路由：
//   - GET /        → ui/index.html（首页入口）
//   - GET /ui/*    → ui/ 下其它静态文件（css/js/vendor）
//
// 资源来自 main 注入的 uiAssets（//go:embed ui）。为空 FS 时跳过挂载。
func registerStaticRoutes(mux *http.ServeMux) {
	if isUIAssetsEmpty() {
		return
	}

	sub, err := fs.Sub(uiAssets, uiRoot)
	if err != nil {
		// embed 声明与 uiRoot 不匹配才会走到这里，属编译期可发现的接线错误
		panic("解析 ui embed 子树失败: " + err.Error())
	}

	// /ui/* —— 文件服务器喂 ui/ 子树
	fileServer := http.FileServer(http.FS(sub))
	mux.Handle("/ui/", http.StripPrefix("/ui/", fileServer))

	// / —— 首页（cube 当前无前端路由，根路径直接返回 index.html）
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 标准库 mux 的 "/" 是 catch-all 兜底；只对真正的根路径与未匹配的非 api 路径回 index
		if r.URL.Path == "/" {
			serveIndex(w, r, sub)
			return
		}
		http.NotFound(w, r)
	})
}

// serveIndex 返回 ui/index.html。
func serveIndex(w http.ResponseWriter, r *http.Request, sub fs.FS) {
	data, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// isUIAssetsEmpty 检测 uiAssets 是否被注入。
// embed.FS 没有 IsEmpty，用读 ui 根目录下 index.html 是否存在判断。
func isUIAssetsEmpty() bool {
	_, err := fs.ReadFile(uiAssets, uiRoot+"/index.html")
	return err != nil
}
